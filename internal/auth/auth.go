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
	if len(password) < 10 {
		return "", false, fmt.Errorf("password must be at least 10 characters")
	}
	rows, e := tx.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: strings.ToLower(email)}, Limit: 1})
	if e != nil {
		return "", false, e
	}
	if len(rows) > 0 {
		return fmt.Sprint(rows[0]["id"]), false, nil
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return "", false, e
	}
	roles, _ := json.Marshal(roleValues)
	id := uid.New()
	_, e = tx.Insert(ctx, dbal.Insert{Table: "bean_user", Values: map[string]dbal.Value{"id": id, "email": strings.ToLower(email), "password_hash": string(hash), "roles": string(roles), "tenant_id": tenantID, "created_at": time.Now().UTC().Format(time.RFC3339Nano)}})
	return id, e == nil, e
}
func (s Service) Login(ctx context.Context, email, password string) (Session, error) {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: strings.ToLower(email)}, Limit: 1})
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
	_, e = s.DB.Insert(ctx, dbal.Insert{Table: "bean_session", Values: map[string]dbal.Value{"id": session.ID, "user_id": session.User.ID, "csrf_token": session.CSRF, "expires_at": session.Expires.Format(time.RFC3339Nano)}})
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
	return Session{ID: uid.New(), CSRF: uid.New(), User: beanctx.User{ID: fmt.Sprint(row["id"]), Email: fmt.Sprint(row["email"]), Roles: roles}, TenantID: fmt.Sprint(row["tenant_id"])}
}
