package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/uid"
	"golang.org/x/crypto/bcrypt"
)

type Session struct {
	ID, CSRF string
	Expires  time.Time
	User     beanctx.User
	TenantID string
}
type Service struct {
	DB  dbal.Database
	TTL time.Duration
}

func (s Service) Bootstrap(ctx context.Context, email, password string) error {
	return s.Create(ctx, email, password, []string{"administrator"}, "")
}
func (s Service) Create(ctx context.Context, email, password string, roleValues []string, tenantID string) error {
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		_, _, err := s.CreateInTransaction(ctx, tx, email, password, roleValues, tenantID)
		return err
	})
}

func (s Service) CreateInTransaction(ctx context.Context, tx dbal.Transaction, email, password string, roleValues []string, tenantID string) (string, bool, error) {
	return s.createInTransaction(ctx, tx, "", email, password, roleValues, tenantID, false)
}

func (s Service) RegisterInTransaction(ctx context.Context, tx dbal.Transaction, displayName, email, password, confirmation, roleValue string) (dbal.Row, error) {
	if password != confirmation {
		return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "password confirmation does not match"}
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "display name is required"}
	}
	id, created, err := s.createInTransaction(ctx, tx, displayName, email, password, []string{roleValue}, "", true)
	if dbal.IsCode(err, dbal.UniqueViolation) {
		err = &dbal.Error{Code: dbal.Conflict, Message: "an account already exists for this email"}
	}
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, &dbal.Error{Code: dbal.Conflict, Message: "an account already exists for this email"}
	}
	return dbal.Row{"id": id, "display_name": displayName, "email": normalizeEmail(email)}, nil
}

func (s Service) createInTransaction(ctx context.Context, tx dbal.Transaction, displayName, email, password string, roleValues []string, tenantID string, duplicateIsConflict bool) (string, bool, error) {
	if len(password) < 10 {
		return "", false, &dbal.Error{Code: dbal.InvalidQuery, Message: "password must be at least 10 characters"}
	}
	email = normalizeEmail(email)
	rows, e := tx.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: email}, Limit: 1})
	if e != nil {
		return "", false, e
	}
	if len(rows) > 0 {
		if duplicateIsConflict {
			return "", false, &dbal.Error{Code: dbal.Conflict, Message: "an account already exists for this email"}
		}
		return fmt.Sprint(rows[0]["id"]), false, nil
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return "", false, e
	}
	roles, _ := json.Marshal(roleValues)
	id := uid.New()
	_, e = tx.Insert(ctx, dbal.Insert{Table: "bean_user", Values: map[string]dbal.Value{"id": id, "email": email, "display_name": nullable(displayName), "password_hash": string(hash), "roles": string(roles), "tenant_id": nullable(tenantID), "created_at": time.Now().UTC().Format(time.RFC3339Nano)}})
	return id, e == nil, e
}
func (s Service) Login(ctx context.Context, email, password string) (Session, error) {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: normalizeEmail(email)}, Limit: 1})
	if e != nil || len(rows) != 1 {
		return Session{}, &dbal.Error{Code: dbal.NotFound, Message: "invalid email or password"}
	}
	if e = bcrypt.CompareHashAndPassword([]byte(fmt.Sprint(rows[0]["password_hash"])), []byte(password)); e != nil {
		return Session{}, &dbal.Error{Code: dbal.NotFound, Message: "invalid email or password"}
	}
	session := sessionFromUser(rows[0])
	if s.TTL == 0 {
		s.TTL = 24 * time.Hour
	}
	session.Expires = time.Now().UTC().Add(s.TTL)
	e = s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		if err := LockUser(ctx, tx, rows[0]); err != nil {
			return err
		}
		_, err := tx.Insert(ctx, dbal.Insert{Table: "bean_session", Values: map[string]dbal.Value{"id": session.ID, "user_id": session.User.ID, "csrf_token": session.CSRF, "expires_at": session.Expires.Format(time.RFC3339Nano)}})
		return err
	})
	return session, e
}
func (s Service) Current(ctx context.Context, id string) (Session, error) {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_session", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if e != nil || len(rows) == 0 {
		return Session{}, &dbal.Error{Code: dbal.NotFound, Message: "session not found"}
	}
	expires, e := time.Parse(time.RFC3339Nano, fmt.Sprint(rows[0]["expires_at"]))
	if e != nil || expires.Before(time.Now()) {
		return Session{}, &dbal.Error{Code: dbal.NotFound, Message: "session expired"}
	}
	users, e := s.DB.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: rows[0]["user_id"]}, Limit: 1})
	if e != nil || len(users) == 0 {
		return Session{}, &dbal.Error{Code: dbal.NotFound, Message: "user not found"}
	}
	out := sessionFromUser(users[0])
	out.ID = id
	out.CSRF = fmt.Sprint(rows[0]["csrf_token"])
	out.Expires = expires
	return out, nil
}
func (s Service) Logout(ctx context.Context, id string) error {
	_, e := s.DB.Delete(ctx, dbal.Delete{Table: "bean_session", Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}})
	return e
}
func sessionFromUser(row dbal.Row) Session {
	roles := []string{}
	_ = json.Unmarshal([]byte(fmt.Sprint(row["roles"])), &roles)
	return Session{ID: uid.New(), CSRF: uid.New(), User: beanctx.User{ID: fmt.Sprint(row["id"]), Email: fmt.Sprint(row["email"]), DisplayName: dbString(row["display_name"]), Roles: roles}, TenantID: dbString(row["tenant_id"])}
}

func normalizeEmail(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func dbString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
