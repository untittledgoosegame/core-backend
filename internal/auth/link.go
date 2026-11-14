package auth

import (
	"context"
	"encoding/base64"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

type UnlinkRequest struct {
	Method       string `json:"method"`
	CredentialID string `json:"credential_id,omitempty"`
}

var methodLabels = map[string]string{
	MethodEmail:      "Email",
	MethodPasskey:    "A passkey",
	MethodTruecaller: "Truecaller",
}

// Link adds another sign-in method to the signed-in account using the same steps as Register.
func (s *Service) Link(ctx context.Context, userID bson.ObjectID, req Request) (*Result, error) {
	var (
		res *Result
		err error
	)
	switch req.Method {
	case MethodEmail:
		res, err = s.linkEmail(ctx, userID, req)
	case MethodPasskey:
		res, err = s.linkPasskey(ctx, userID, req)
	case MethodTruecaller:
		res, err = s.linkTruecaller(ctx, userID, req)
	default:
		return nil, errUnknownMethod
	}
	if err == nil && res.User != nil {
		s.alert(res.User.Email, "Sign-in method added", methodLabels[req.Method]+" can now be used to sign in to your account.")
	}
	return res, err
}

func (s *Service) Unlink(ctx context.Context, userID bson.ObjectID, req UnlinkRequest) (*user.User, error) {
	before, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var after *user.User
	switch req.Method {
	case MethodEmail:
		after, err = s.users.UnlinkEmail(ctx, userID)
	case MethodTruecaller:
		after, err = s.users.UnlinkTruecaller(ctx, userID)
	case MethodPasskey:
		credID, decodeErr := base64.RawURLEncoding.DecodeString(req.CredentialID)
		if decodeErr != nil || len(credID) == 0 {
			return nil, apperr.BadRequest("credential_id must be a base64url passkey id")
		}
		after, err = s.users.RemovePasskey(ctx, userID, credID)
	default:
		return nil, errUnknownMethod
	}
	if err != nil {
		return nil, err
	}
	s.alert(before.Email, "Sign-in method removed", methodLabels[req.Method]+" can no longer be used to sign in to your account.")
	return after, nil
}
