package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/demoseed"
)

const packageAPIVersion = "bean.package/v1alpha1"

type packageManifest struct {
	APIVersion       string `json:"apiVersion"`
	BeanVersion      string `json:"beanVersion"`
	SourceChecksum   string `json:"sourceChecksum"`
	SeedChecksum     string `json:"seedChecksum"`
	BinaryChecksum   string `json:"binaryChecksum"`
	DatabaseChecksum string `json:"databaseChecksum"`
	Release          struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	} `json:"release"`
	Start []string `json:"start"`
}

type packageResult struct {
	Output   string          `json:"output"`
	Manifest packageManifest `json:"manifest"`
}

func agentPackageBuild(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("package", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "", "application manifest")
	output := flags.String("output", "", "package directory")
	seed := flags.Int64("seed", 1, "deterministic seed")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *filename == "" || *output == "" {
		message := "--file and --output are required"
		if err != nil {
			message = err.Error()
		} else if flags.NArg() != 0 {
			message = "unexpected arguments: " + strings.Join(flags.Args(), " ")
		}
		return writeCommandFailure("package", message, exitUsage, jsonOutput, stdout, stderr)
	}
	bundle, diagnostics := validateSource(*filename)
	if len(diagnostics) > 0 {
		return writeSourceFailure("package", *filename, diagnostics, jsonOutput, stdout, stderr)
	}
	result, err := buildPackage(bundleChecksum(bundle), bundle.Name, bundle.Definitions, *output, *seed)
	if err != nil {
		return writeRuntimeFailure("package", err, jsonOutput, stdout, stderr)
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "package", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintln(stdout, result.Output)
	}
	return exitOK
}

func agentPackageVerify(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("package verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	directory := flags.String("dir", "", "package directory")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *directory == "" {
		message := "--dir is required"
		if err != nil {
			message = err.Error()
		}
		return writeCommandFailure("package.verify", message, exitUsage, jsonOutput, stdout, stderr)
	}
	manifest, err := verifyPackage(*directory)
	if err != nil {
		return writeRuntimeFailure("package.verify", err, jsonOutput, stdout, stderr)
	}
	result := map[string]any{"directory": filepath.Clean(*directory), "manifest": manifest}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "package.verify", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintln(stdout, "package verified")
	}
	return exitOK
}

func buildPackage(sourceChecksum, name string, definitions []definition.Definition, output string, seed int64) (packageResult, error) {
	cleanOutput, err := safePackageOutput(output)
	if err != nil {
		return packageResult{}, err
	}
	parent := filepath.Dir(cleanOutput)
	if err = os.MkdirAll(parent, 0o755); err != nil {
		return packageResult{}, err
	}
	stage, err := os.MkdirTemp(parent, ".bean-package-")
	if err != nil {
		return packageResult{}, err
	}
	defer os.RemoveAll(stage)
	database := filepath.Join(stage, "bean.db")
	runtime, err := bootstrap.Open(context.Background(), database, false)
	if err != nil {
		return packageResult{}, err
	}
	bundle := definition.Bundle{Name: name, Definitions: definitions}
	published, _, diagnostics, err := runtime.Store.PublishBundle(context.Background(), "default", bundle)
	if err == nil && len(diagnostics) > 0 {
		err = diagnostics[0]
	}
	var seeded demoseed.Result
	if err == nil {
		app, active := runtime.Kernel.Active()
		if !active {
			err = fmt.Errorf("published application did not activate")
		} else {
			seeded, err = demoseed.Run(context.Background(), runtime.DB, app, seed)
		}
	}
	closeErr := runtime.DB.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return packageResult{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return packageResult{}, err
	}
	packagedExecutable := filepath.Join(stage, "bean")
	if err = copyFile(executable, packagedExecutable, 0o755); err != nil {
		return packageResult{}, err
	}
	manifest := packageManifest{APIVersion: packageAPIVersion, BeanVersion: version, SourceChecksum: sourceChecksum, SeedChecksum: seeded.Checksum, Start: []string{"./bean", "serve", "--db", "./bean.db", "--addr", "127.0.0.1:8080"}}
	manifest.Release.ID, manifest.Release.Version = published.ID, published.Version
	if manifest.BinaryChecksum, err = fileChecksum(packagedExecutable); err != nil {
		return packageResult{}, err
	}
	if manifest.DatabaseChecksum, err = fileChecksum(database); err != nil {
		return packageResult{}, err
	}
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	encoded = append(encoded, '\n')
	if err = os.WriteFile(filepath.Join(stage, "manifest.json"), encoded, 0o644); err != nil {
		return packageResult{}, err
	}
	restarted, err := bootstrap.Open(context.Background(), database, false)
	if err != nil {
		return packageResult{}, fmt.Errorf("package restart check: %w", err)
	}
	active, ok := restarted.Kernel.Active()
	restartCloseErr := restarted.DB.Close()
	if !ok || active.ReleaseID != published.ID {
		return packageResult{}, fmt.Errorf("package restart check loaded the wrong release")
	}
	if restartCloseErr != nil {
		return packageResult{}, restartCloseErr
	}
	if err = replaceDirectory(stage, cleanOutput); err != nil {
		return packageResult{}, err
	}
	return packageResult{Output: cleanOutput, Manifest: manifest}, nil
}

func verifyPackage(directory string) (packageManifest, error) {
	manifestPath := filepath.Join(directory, "manifest.json")
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		return packageManifest{}, err
	}
	var manifest packageManifest
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return packageManifest{}, err
	}
	if manifest.APIVersion != packageAPIVersion {
		return packageManifest{}, fmt.Errorf("unsupported package format %q", manifest.APIVersion)
	}
	for _, item := range []struct{ name, expected string }{{"bean", manifest.BinaryChecksum}, {"bean.db", manifest.DatabaseChecksum}} {
		actual, checksumErr := fileChecksum(filepath.Join(directory, item.name))
		if checksumErr != nil {
			return packageManifest{}, checksumErr
		}
		if actual != item.expected {
			return packageManifest{}, fmt.Errorf("checksum mismatch for %s", item.name)
		}
	}
	runtime, err := bootstrap.Open(context.Background(), filepath.Join(directory, "bean.db"), false)
	if err != nil {
		return packageManifest{}, err
	}
	defer runtime.DB.Close()
	active, ok := runtime.Kernel.Active()
	if !ok || active.ReleaseID != manifest.Release.ID || active.Version != manifest.Release.Version {
		return packageManifest{}, fmt.Errorf("active release does not match package manifest")
	}
	return manifest, nil
}

func safePackageOutput(output string) (string, error) {
	abs, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(abs)
	workingDirectory, _ := os.Getwd()
	userDirectory, _ := os.UserHomeDir()
	if clean == string(filepath.Separator) || clean == filepath.Dir(clean) || clean == filepath.Clean(workingDirectory) || clean == filepath.Clean(userDirectory) {
		return "", fmt.Errorf("unsafe package output %q", output)
	}
	return clean, nil
}

func replaceDirectory(stage, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		encoded, readErr := os.ReadFile(filepath.Join(destination, "manifest.json"))
		var existing struct {
			APIVersion string `json:"apiVersion"`
		}
		if readErr != nil || json.Unmarshal(encoded, &existing) != nil || existing.APIVersion != packageAPIVersion {
			return fmt.Errorf("refusing to replace a directory that is not a Bean package: %s", destination)
		}
		backup, backupErr := os.MkdirTemp(filepath.Dir(destination), ".bean-package-backup-")
		if backupErr != nil {
			return backupErr
		}
		if removeErr := os.Remove(backup); removeErr != nil {
			return removeErr
		}
		if err = os.Rename(destination, backup); err != nil {
			return err
		}
		if err = os.Rename(stage, destination); err != nil {
			_ = os.Rename(backup, destination)
			return err
		}
		return os.RemoveAll(backup)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(stage, destination)
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func fileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
