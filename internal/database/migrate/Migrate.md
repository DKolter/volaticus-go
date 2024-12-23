# Database Migrations

This package handles database migrations for the application. It uses `golang-migrate/migrate` for managing database schema changes.

## Structure

```bash
migrations/
├── 000001_create_users_table.up.sql    # Creates users table
└── 000001_create_users_table.down.sql   # Rolls back users table creation
```

## Usage

Migrations are automatically run when the application starts.

## Adding New Migrations

To add a new migration:

1. Create two new files in the migrations directory:

   - `{version}_{name}.up.sql` for the migration
   - `{version}_{name}.down.sql` for the rollback

2. Version numbers should be sequential and zero-padded (e.g., 000001, 000002)

## Migration Naming Convention

Migration files should:

- Use lowercase with underscores
- Be descriptive but concise
- Include both up and down migrations
- Follow the format: `{version}_{name}.{up|down}.sql`

Example: `000002_add_user_roles.up.sql`
