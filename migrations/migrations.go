package migrations

import "gofr.dev/pkg/gofr/migration"

// All returns all database migrations for the LLM gateway.
func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		1:  createBudgetsTable(),
		2:  createSpendLogTable(),
		3:  createVirtualKeysTable(),
		4:  createBlockedUsersTable(),
		5:  createTeamsTable(),
		6:  createUsersTable(),
		7:  createOrganizationsTable(),
		8:  createAuditLogTable(),
		9:  createGuardrailConfigsTable(),
		10: createBatchesTable(),
		11: createBatchItemsTable(),
		12: addOrgAdminEmail(),
		13: createProviderConfigTable(),
		14: addUserRoleAndOrgConstraints(),
		15: createGatewayFilesTable(),
		16: createFineTuningJobsTable(),
		17: createFineTuningEventsTable(),
		18: createAssistantsTable(),
		19: createThreadsTable(),
		20: createThreadMessagesTable(),
		21: createRunsTable(),
	}
}

func addOrgAdminEmail() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`ALTER TABLE organizations ADD COLUMN IF NOT EXISTS admin_email VARCHAR(255) DEFAULT ''`)
			return err
		},
	}
}

func createGuardrailConfigsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS guardrail_configs (
				id SERIAL PRIMARY KEY,
				key_hash VARCHAR(64) UNIQUE,
				max_input_tokens INT DEFAULT 0,
				max_output_tokens INT DEFAULT 0,
				blocked_keywords TEXT,
				pii_action VARCHAR(20) DEFAULT 'none',
				enabled BOOLEAN DEFAULT TRUE,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createBatchesTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS batches (
				id VARCHAR(36) PRIMARY KEY,
				status VARCHAR(20) DEFAULT 'pending',
				total_requests INT NOT NULL,
				completed_requests INT DEFAULT 0,
				failed_requests INT DEFAULT 0,
				key_hash VARCHAR(64),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				completed_at TIMESTAMP
			)`)
			return err
		},
	}
}

func createBatchItemsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS batch_items (
				id SERIAL PRIMARY KEY,
				batch_id VARCHAR(36) NOT NULL REFERENCES batches(id),
				custom_id VARCHAR(255),
				method VARCHAR(10) DEFAULT 'POST',
				url VARCHAR(255),
				body TEXT,
				status VARCHAR(20) DEFAULT 'pending',
				status_code INT,
				result TEXT,
				error TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				completed_at TIMESTAMP
			)`)
			return err
		},
	}
}

func createBudgetsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS budgets (
				id SERIAL PRIMARY KEY,
				entity_type VARCHAR(50) NOT NULL,
				entity_id VARCHAR(255) NOT NULL,
				max_budget DECIMAL(10,4) NOT NULL DEFAULT 0,
				current_spend DECIMAL(10,4) NOT NULL DEFAULT 0,
				reset_period VARCHAR(20) NOT NULL DEFAULT 'monthly',
				reset_at TIMESTAMP,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(entity_type, entity_id)
			)`)
			return err
		},
	}
}

func createSpendLogTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS spend_log (
				id SERIAL PRIMARY KEY,
				key_id VARCHAR(255),
				user_id VARCHAR(255),
				team_id VARCHAR(255),
				org_id VARCHAR(255),
				provider VARCHAR(100) NOT NULL,
				model VARCHAR(255) NOT NULL,
				prompt_tokens INT NOT NULL DEFAULT 0,
				completion_tokens INT NOT NULL DEFAULT 0,
				total_tokens INT NOT NULL DEFAULT 0,
				cost DECIMAL(10,6) NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createVirtualKeysTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS virtual_keys (
				id SERIAL PRIMARY KEY,
				key_hash VARCHAR(64) NOT NULL UNIQUE,
				key_prefix VARCHAR(20) NOT NULL,
				name VARCHAR(255) NOT NULL,
				team_id VARCHAR(255),
				user_id VARCHAR(255),
				org_id VARCHAR(255),
				allowed_models TEXT,
				max_budget DECIMAL(10,4) DEFAULT 0,
				rate_limit_rpm INT DEFAULT 0,
				rate_limit_tpm INT DEFAULT 0,
				tier VARCHAR(50) DEFAULT 'default',
				tags TEXT,
				expires_at TIMESTAMP,
				is_active BOOLEAN NOT NULL DEFAULT TRUE,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createBlockedUsersTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS blocked_users (
				id SERIAL PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL UNIQUE,
				reason TEXT,
				blocked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createTeamsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS teams (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				org_id VARCHAR(255),
				max_budget DECIMAL(10,4) DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createUsersTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS users (
				id SERIAL PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL UNIQUE,
				email VARCHAR(255),
				team_id VARCHAR(255),
				org_id VARCHAR(255),
				max_budget DECIMAL(10,4) DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createOrganizationsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS organizations (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				max_budget DECIMAL(10,4) DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createProviderConfigTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS provider_config (
				id SERIAL PRIMARY KEY,
				provider_name VARCHAR(50) NOT NULL UNIQUE,
				api_key TEXT,
				base_url TEXT,
				timeout_ms INT DEFAULT 0,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

func createAuditLogTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
				id SERIAL PRIMARY KEY,
				action VARCHAR(100) NOT NULL,
				entity_type VARCHAR(50) NOT NULL,
				entity_id VARCHAR(255) NOT NULL,
				actor_id VARCHAR(255),
				details TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
			return err
		},
	}
}

// addUserRoleAndOrgConstraints adds role column to users and ensures
// a default organization exists for backfilling empty org_id references.
func addUserRoleAndOrgConstraints() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			// Add role column to users (admin or member)
			_, err := d.SQL.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) DEFAULT 'member'`)
			if err != nil {
				return err
			}

			// Ensure a default organization exists
			_, err = d.SQL.Exec(`INSERT INTO organizations (name, admin_email, max_budget)
				SELECT 'default', '', 0 WHERE NOT EXISTS (SELECT 1 FROM organizations)`)
			if err != nil {
				return err
			}

			// Backfill empty org_id on teams with the default org
			_, err = d.SQL.Exec(`UPDATE teams SET org_id = (SELECT CAST(id AS VARCHAR) FROM organizations ORDER BY id LIMIT 1) WHERE org_id IS NULL OR org_id = ''`)
			if err != nil {
				return err
			}

			// Backfill empty org_id on users
			_, err = d.SQL.Exec(`UPDATE users SET org_id = (SELECT CAST(id AS VARCHAR) FROM organizations ORDER BY id LIMIT 1) WHERE org_id IS NULL OR org_id = ''`)
			if err != nil {
				return err
			}

			// Backfill empty org_id on virtual_keys
			_, err = d.SQL.Exec(`UPDATE virtual_keys SET org_id = (SELECT CAST(id AS VARCHAR) FROM organizations ORDER BY id LIMIT 1) WHERE org_id IS NULL OR org_id = ''`)
			return err
		},
	}
}

func createGatewayFilesTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS gateway_files (
				id VARCHAR(64) PRIMARY KEY,
				filename VARCHAR(512) NOT NULL,
				purpose VARCHAR(64) NOT NULL DEFAULT 'assistants',
				bytes BIGINT NOT NULL DEFAULT 0,
				content_b64 TEXT,
				created_at BIGINT NOT NULL DEFAULT 0
			)`)
			return err
		},
	}
}

func createFineTuningJobsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS fine_tuning_jobs (
				id VARCHAR(64) PRIMARY KEY,
				model VARCHAR(255) NOT NULL,
				training_file VARCHAR(255) NOT NULL,
				validation_file VARCHAR(255),
				hyperparameters TEXT DEFAULT '{}',
				status VARCHAR(50) DEFAULT 'queued',
				provider VARCHAR(100),
				fine_tuned_model VARCHAR(255),
				trained_tokens INT,
				created_at BIGINT NOT NULL DEFAULT 0,
				finished_at BIGINT
			)`)
			return err
		},
	}
}

func createFineTuningEventsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS fine_tuning_events (
				id VARCHAR(64) PRIMARY KEY,
				job_id VARCHAR(64) NOT NULL REFERENCES fine_tuning_jobs(id),
				level VARCHAR(20) DEFAULT 'info',
				message TEXT,
				created_at BIGINT NOT NULL DEFAULT 0
			)`)
			return err
		},
	}
}

func createAssistantsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS assistants (
				id VARCHAR(64) PRIMARY KEY,
				name VARCHAR(255),
				description TEXT,
				model VARCHAR(255) NOT NULL,
				instructions TEXT,
				tools TEXT DEFAULT '[]',
				metadata TEXT DEFAULT '{}',
				created_at BIGINT NOT NULL DEFAULT 0
			)`)
			return err
		},
	}
}

func createThreadsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS threads (
				id VARCHAR(64) PRIMARY KEY,
				metadata TEXT DEFAULT '{}',
				created_at BIGINT NOT NULL DEFAULT 0
			)`)
			return err
		},
	}
}

func createThreadMessagesTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS thread_messages (
				id VARCHAR(64) PRIMARY KEY,
				thread_id VARCHAR(64) NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
				role VARCHAR(20) NOT NULL DEFAULT 'user',
				content TEXT NOT NULL DEFAULT '[]',
				assistant_id VARCHAR(64),
				run_id VARCHAR(64),
				created_at BIGINT NOT NULL DEFAULT 0
			)`)
			return err
		},
	}
}

func createRunsTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(`CREATE TABLE IF NOT EXISTS runs (
				id VARCHAR(64) PRIMARY KEY,
				thread_id VARCHAR(64) NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
				assistant_id VARCHAR(64) NOT NULL,
				model VARCHAR(255) NOT NULL,
				instructions TEXT,
				tools TEXT DEFAULT '[]',
				status VARCHAR(50) DEFAULT 'queued',
				created_at BIGINT NOT NULL DEFAULT 0,
				started_at BIGINT,
				completed_at BIGINT,
				failed_at BIGINT
			)`)
			return err
		},
	}
}
