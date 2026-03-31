// scripts/mongo-init.js
// Runs once when the MongoDB container is first initialized.
// Creates the database, collection, and indexes.

db = db.getSiblingDB('ai_readiness');

db.createCollection('assessments');

db.assessments.createIndex({ created_at: -1 });
db.assessments.createIndex({ status: 1 });
db.assessments.createIndex({ client_ref: 1 }, { sparse: true });

print('ai_readiness database initialized');
