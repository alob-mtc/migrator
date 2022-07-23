-- Create Table 
CREATE TABLE "test1" ("id" text,"created_at" timestamptz,"updated_at" timestamptz,"deleted_at" timestamptz,"name" text NOT NULL,"active" boolean NOT NULL DEFAULT false,PRIMARY KEY ("id")); 
CREATE INDEX "idx_test1_deleted_at" ON "test1"("deleted_at"); 
CREATE INDEX "idx_test1_active" ON "test1"("active"); 
CREATE INDEX "idx_test1_name" ON "test1"("name"); 

-- Create Table 
CREATE TABLE "test2" ("id" text,"created_at" timestamptz,"deleted_at" timestamptz,"name" text NOT NULL,"active" boolean DEFAULT false,"partner_id" text NOT NULL UNIQUE,PRIMARY KEY ("id")); 
CREATE INDEX "idx_test2_partner_id" ON "test2"("partner_id"); 
CREATE INDEX "idx_test2_name" ON "test2"("name"); 
CREATE INDEX "idx_test2_deleted_at" ON "test2"("deleted_at"); 

