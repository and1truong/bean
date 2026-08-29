# ADR 0001: Modular monolith

Status: accepted. Keep module boundaries in Go packages and compose them in `internal/bootstrap`; deploy one process to preserve simple transactions and operations.
