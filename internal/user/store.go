package user

import (
	"context"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

var (
	ErrNotFound    = apperr.NotFound("account not found")
	errNotLinked   = apperr.NotFound("that sign-in method isn't linked to your account")
	errLastMethod  = apperr.Conflict("you can't remove your only sign-in method")
	errEmailInUse  = apperr.Conflict("this email belongs to another account")
	errTCInUse     = apperr.Conflict("this truecaller profile belongs to another account")
	errEmailLinked = apperr.Conflict("an email is already linked; unlink it first")
	errTCLinked    = apperr.Conflict("a truecaller profile is already linked; unlink it first")
)

func exists(field string) bson.D {
	return bson.D{{Key: field, Value: bson.D{{Key: "$exists", Value: true}}}}
}

func missing(field string) bson.D {
	return bson.D{{Key: field, Value: bson.D{{Key: "$exists", Value: false}}}}
}

type Store struct {
	users *mongo.Collection
}

func NewStore(db *mongo.Database) *Store {
	return &Store{users: db.Collection("users")}
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	sparseUnique := func(field string) mongo.IndexModel {
		return mongo.IndexModel{
			Keys:    bson.D{{Key: field, Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(exists(field)),
		}
	}
	_, err := s.users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		sparseUnique("email"),
		sparseUnique("truecaller_id"),
		sparseUnique("profile.username"),
		{Keys: bson.D{{Key: "webauthn_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	return err
}

func (s *Store) Create(ctx context.Context, u *User) error {
	now := time.Now().UTC()
	if u.ID.IsZero() {
		u.ID = bson.NewObjectID()
	}
	u.CreatedAt, u.UpdatedAt = now, now
	_, err := s.users.InsertOne(ctx, u)
	if mongo.IsDuplicateKeyError(err) {
		return apperr.Conflict("account already exists, log in instead")
	}
	return err
}

func (s *Store) findOne(ctx context.Context, filter bson.D) (*User, error) {
	var u User
	err := s.users.FindOne(ctx, filter).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) ByID(ctx context.Context, id bson.ObjectID) (*User, error) {
	return s.findOne(ctx, bson.D{{Key: "_id", Value: id}})
}

func (s *Store) ByEmail(ctx context.Context, email string) (*User, error) {
	return s.findOne(ctx, bson.D{{Key: "email", Value: email}})
}

func (s *Store) ByTruecallerID(ctx context.Context, id string) (*User, error) {
	return s.findOne(ctx, bson.D{{Key: "truecaller_id", Value: id}})
}

func (s *Store) ByWebAuthnID(ctx context.Context, id []byte) (*User, error) {
	return s.findOne(ctx, bson.D{{Key: "webauthn_id", Value: id}})
}

func (s *Store) update(ctx context.Context, filter bson.D, change bson.D) (*User, error) {
	change = append(change, bson.E{Key: "$currentDate", Value: bson.D{{Key: "updated_at", Value: true}}})
	var u User
	err := s.users.FindOneAndUpdate(ctx, filter, change,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func byID(id bson.ObjectID, extra ...bson.E) bson.D {
	return append(bson.D{{Key: "_id", Value: id}}, extra...)
}

func set(fields bson.D) bson.D { return bson.D{{Key: "$set", Value: fields}} }

func (s *Store) SetProfile(ctx context.Context, id bson.ObjectID, p Profile) (*User, error) {
	u, err := s.update(ctx, byID(id), set(bson.D{{Key: "profile", Value: p}}))
	if mongo.IsDuplicateKeyError(err) {
		return nil, apperr.Conflict("username is taken")
	}
	return u, err
}

func (s *Store) SetTOTP(ctx context.Context, id bson.ObjectID, t TOTP) (*User, error) {
	return s.update(ctx, byID(id), set(bson.D{{Key: "totp", Value: t}}))
}

func (s *Store) UpdatePasskey(ctx context.Context, id bson.ObjectID, cred webauthn.Credential) error {
	_, err := s.update(ctx,
		byID(id, bson.E{Key: "passkeys.id", Value: cred.ID}),
		set(bson.D{{Key: "passkeys.$", Value: cred}}))
	return err
}

func (s *Store) AddPasskey(ctx context.Context, id bson.ObjectID, cred webauthn.Credential) (*User, error) {
	return s.update(ctx, byID(id), bson.D{{Key: "$push", Value: bson.D{{Key: "passkeys", Value: cred}}}})
}

func (s *Store) LinkEmail(ctx context.Context, id bson.ObjectID, email string) (*User, error) {
	u, err := s.update(ctx, byID(id, missing("email")...), set(bson.D{{Key: "email", Value: email}}))
	switch {
	case mongo.IsDuplicateKeyError(err):
		return nil, errEmailInUse
	case errors.Is(err, ErrNotFound):
		return nil, errEmailLinked
	}
	return u, err
}

func (s *Store) LinkTruecaller(ctx context.Context, id bson.ObjectID, truecallerID, phone string) (*User, error) {
	u, err := s.update(ctx, byID(id, missing("truecaller_id")...),
		set(bson.D{{Key: "truecaller_id", Value: truecallerID}, {Key: "phone", Value: phone}}))
	switch {
	case mongo.IsDuplicateKeyError(err):
		return nil, errTCInUse
	case errors.Is(err, ErrNotFound):
		return nil, errTCLinked
	}
	return u, err
}

// Unlinking is a single conditional update so concurrent requests can never strip the last method.
func (s *Store) unlink(ctx context.Context, id bson.ObjectID, present bson.E, otherMethods []bson.D, change bson.D) (*User, error) {
	u, err := s.update(ctx, byID(id, present, bson.E{Key: "$or", Value: otherMethods}), change)
	if !errors.Is(err, ErrNotFound) {
		return u, err
	}
	n, countErr := s.users.CountDocuments(ctx, byID(id, present))
	if countErr != nil {
		return nil, countErr
	}
	if n == 0 {
		return nil, errNotLinked
	}
	return nil, errLastMethod
}

func (s *Store) UnlinkEmail(ctx context.Context, id bson.ObjectID) (*User, error) {
	return s.unlink(ctx, id, exists("email")[0],
		[]bson.D{exists("truecaller_id"), exists("passkeys.0")},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "email", Value: ""}}}})
}

func (s *Store) UnlinkTruecaller(ctx context.Context, id bson.ObjectID) (*User, error) {
	return s.unlink(ctx, id, exists("truecaller_id")[0],
		[]bson.D{exists("email"), exists("passkeys.0")},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "truecaller_id", Value: ""}, {Key: "phone", Value: ""}}}})
}

func (s *Store) RemovePasskey(ctx context.Context, id bson.ObjectID, credentialID []byte) (*User, error) {
	return s.unlink(ctx, id, bson.E{Key: "passkeys.id", Value: credentialID},
		[]bson.D{exists("email"), exists("truecaller_id"), exists("passkeys.1")},
		bson.D{{Key: "$pull", Value: bson.D{{Key: "passkeys", Value: bson.D{{Key: "id", Value: credentialID}}}}}})
}

func (s *Store) TakenUsernames(ctx context.Context, candidates []string) (map[string]bool, error) {
	cur, err := s.users.Find(ctx,
		bson.D{{Key: "profile.username", Value: bson.D{{Key: "$in", Value: candidates}}}},
		options.Find().SetProjection(bson.D{{Key: "profile.username", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	var docs []User
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	taken := make(map[string]bool, len(docs))
	for _, d := range docs {
		taken[d.Profile.Username] = true
	}
	return taken, nil
}
