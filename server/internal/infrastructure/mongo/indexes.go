package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateMany is a no-op for an existing identical index.
var indexes = map[string][]mongo.IndexModel{
	mailOutboxCollection: {
		{Keys: bson.D{{Key: "state", Value: 1}, {Key: "nextAttemptAt", Value: 1}}, Options: options.Index().SetName("mail_outbox_state_next_attempt")},
		{Keys: bson.D{{Key: "state", Value: 1}, {Key: "claim.until", Value: 1}}, Options: options.Index().SetName("mail_outbox_state_claim_until")},
		{Keys: bson.D{{Key: "state", Value: 1}, {Key: "createdAt", Value: 1}}, Options: options.Index().SetName("mail_outbox_state_created")},
		{
			Keys:    bson.D{{Key: "endedAt", Value: 1}},
			Options: options.Index().SetName("mail_outbox_ended").SetPartialFilterExpression(bson.M{"endedAt": bson.M{"$exists": true}}),
		},
		{
			Keys:    bson.D{{Key: "user", Value: 1}},
			Options: options.Index().SetName("mail_outbox_user").SetPartialFilterExpression(bson.M{"user": bson.M{"$exists": true}}),
		},
		{
			Keys:    bson.D{{Key: "subject.notice", Value: 1}, {Key: "state", Value: 1}},
			Options: options.Index().SetName("mail_outbox_notice_state").SetPartialFilterExpression(bson.M{"subject.notice": bson.M{"$exists": true}}),
		},
	},
	policyAcceptancesCollection: {{
		Keys:    bson.D{{Key: "user", Value: 1}, {Key: "type", Value: 1}, {Key: "version", Value: 1}},
		Options: options.Index().SetName("policy_acceptances_user_type_version").SetUnique(true),
	}},
	policyNoticesCollection: {
		{
			Keys:    bson.D{{Key: "type", Value: 1}, {Key: "version", Value: 1}, {Key: "audience", Value: 1}},
			Options: options.Index().SetName("policy_notices_type_version_audience").SetUnique(true),
		},
		{Keys: bson.D{{Key: "fanout.done", Value: 1}, {Key: "createdAt", Value: 1}}, Options: options.Index().SetName("policy_notices_fanout")},
	},
	policyVersionsCollection: {{
		Keys:    bson.D{{Key: "type", Value: 1}, {Key: "version", Value: 1}},
		Options: options.Index().SetName("policy_versions_type_version").SetUnique(true),
	}},
}

func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	for coll, models := range indexes {
		if _, err := db.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create indexes on %s: %w", coll, err)
		}
	}
	return nil
}
