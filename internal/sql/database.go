package sql

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/PGSSoft/terraform-provider-mssql/internal/utils"
	"strings"
)

const NullDatabaseId = DatabaseId(-1)

type DatabaseSettings struct {
	Name      string
	Collation string
	ForceDrop bool
}

type DatabasePermission struct {
	Name            string
	WithGrantOption bool
}

type DatabasePermissions map[string]DatabasePermission

type Database interface {
	GetConnection(context.Context) Connection
	GetId(context.Context) DatabaseId
	Exists(context.Context) bool
	GetSettings(context.Context) DatabaseSettings
	Rename(_ context.Context, name string)
	SetCollation(_ context.Context, collation string)
	Drop(context.Context)
	Query(ctx context.Context, query string) []map[string]string
	Exec(ctx context.Context, script string)
	GetPermissions(ctx context.Context, id GenericDatabasePrincipalId) DatabasePermissions
	GrantPermission(ctx context.Context, id GenericDatabasePrincipalId, permission DatabasePermission)
	UpdatePermission(ctx context.Context, id GenericDatabasePrincipalId, permission DatabasePermission)
	RevokePermission(ctx context.Context, id GenericDatabasePrincipalId, permissionName string)
	connect(context.Context) *sql.DB
	getUserName(ctx context.Context, id GenericDatabasePrincipalId) string
}

type database struct {
	conn Connection
	id   DatabaseId
}

func CreateDatabase(ctx context.Context, conn Connection, settings DatabaseSettings) Database {
	var query strings.Builder
	query.WriteString(fmt.Sprintf("CREATE DATABASE [%s]", settings.Name))

	if settings.Collation != "" {
		query.WriteString(fmt.Sprintf(" COLLATE %s", settings.Collation))
	}

	conn.exec(ctx, query.String())

	if utils.HasError(ctx) {
		return nil
	}

	return GetDatabaseByName(ctx, conn, settings.Name)
}

func GetDatabase(_ context.Context, conn Connection, id DatabaseId) Database {
	return &database{conn: conn, id: id}
}

func GetDatabaseByName(ctx context.Context, conn Connection, name string) Database {
	id := DatabaseId(0)

	if err := conn.getSqlConnection(ctx).QueryRowContext(ctx, "SELECT database_id FROM sys.databases WHERE [name] = @p1", name).Scan(&id); err != nil {
		utils.AddError(ctx, fmt.Sprintf("Failed to retrieve DB ID for name '%s'", name), err)
		return nil
	}

	return GetDatabase(ctx, conn, id)
}

func GetDatabases(ctx context.Context, conn Connection) map[DatabaseId]Database {
	const errorSummary = "Failed to retrieve list of DBs"
	result := map[DatabaseId]Database{}

	switch rows, err := conn.getSqlConnection(ctx).QueryContext(ctx, "SELECT [database_id] FROM sys.databases"); err {
	case sql.ErrNoRows: // ignore
	case nil:
		for rows.Next() {
			var db = database{conn: conn}
			err = rows.Scan(&db.id)
			if err != nil {
				utils.AddError(ctx, errorSummary, err)
			}
			result[db.id] = &db
		}
	default:
		utils.AddError(ctx, errorSummary, err)
	}

	return result
}

func (db *database) GetConnection(context.Context) Connection {
	return db.conn
}

func (db *database) GetId(context.Context) DatabaseId {
	return db.id
}

func (db *database) Exists(ctx context.Context) bool {
	switch _, err := db.getSettingsRaw(ctx); err {
	case sql.ErrNoRows:
		return false
	case nil:
		return true
	default:
		utils.AddError(ctx, "Could not retrieve DB info", err)
		return false
	}
}

func (db *database) GetSettings(ctx context.Context) DatabaseSettings {
	settings, err := db.getSettingsRaw(ctx)

	if err != nil {
		utils.AddError(ctx, "Could not retrieve DB info", err)
	}

	return settings
}

func (db *database) Rename(ctx context.Context, name string) {
	settings := db.GetSettings(ctx)
	db.conn.exec(ctx, fmt.Sprintf("ALTER DATABASE [%s] MODIFY NAME = %s", settings.Name, name))
}

func (db *database) SetCollation(ctx context.Context, collation string) {
	settings := db.GetSettings(ctx)
	db.conn.exec(ctx, fmt.Sprintf("ALTER DATABASE [%s] COLLATE %s", settings.Name, collation))
}

func (db *database) Drop(ctx context.Context) {
	settings := db.GetSettings(ctx)
	if settings.ForceDrop {
		db.conn.exec(ctx, fmt.Sprintf("ALTER DATABASE [%s] SET SINGLE_USER WITH ROLLBACK IMMEDIATE;", settings.Name))	
	}
	db.conn.exec(ctx, fmt.Sprintf("DROP DATABASE [%s]", settings.Name))
}

func (db *database) Query(ctx context.Context, script string) []map[string]string {
	conn := db.connect(ctx)

	if conn == nil {
		return nil
	}

	rows, err := conn.QueryContext(ctx, script)

	if err != nil {
		utils.AddError(ctx, "Failed to execute get state script", err)
		return nil
	}

	cols, err := rows.Columns()
	if err != nil {
		utils.AddError(ctx, "Failed to retrieve names of columns in the script result", err)
		return nil
	}

	values := make([]sql.NullString, len(cols))
	valuePtrs := make([]any, len(cols))
	for i, _ := range values {
		valuePtrs[i] = &values[i]
	}

	var res []map[string]string
	for rows.Next() {
		if err = rows.Scan(valuePtrs...); err != nil {
			utils.AddError(ctx, "Failed to fetch values of the script result", err)
			return nil
		}

		row := map[string]string{}
		for i, name := range cols {
			if values[i].Valid {
				row[name] = values[i].String
			}
		}

		res = append(res, row)
	}

	return res
}

func (db *database) Exec(ctx context.Context, script string) {
	if _, err := db.connect(ctx).ExecContext(ctx, script); err != nil {
		utils.AddError(ctx, "Failed to execute SQL script", err)
	}
}

func (db *database) GetPermissions(ctx context.Context, id GenericDatabasePrincipalId) DatabasePermissions {
	conn := db.connect(ctx)

	if utils.HasError(ctx) {
		return nil
	}

	res, err := conn.
		QueryContext(ctx, "SELECT [permission_name], [state] FROM sys.database_permissions WHERE [class] = 0 AND [state] IN ('G', 'W') AND [grantee_principal_id] = @p1", id)

	perms := DatabasePermissions{}

	switch err {
	case sql.ErrNoRows:
	case nil:
		for res.Next() {
			perm := DatabasePermission{}
			var state string
			err := res.Scan(&perm.Name, &state)
			utils.AddError(ctx, "Failed to parse result", err)
			perm.WithGrantOption = state == "W"
			perms[perm.Name] = perm
		}
	default:
		utils.AddError(ctx, "Failed to retrieve permissions", err)
		return nil
	}

	return perms
}

func (db *database) GrantPermission(ctx context.Context, id GenericDatabasePrincipalId, permission DatabasePermission) {
	conn := db.connect(ctx)
	userName := db.getUserName(ctx, id)

	utils.StopOnError(ctx).
		Then(func() {
			stat := fmt.Sprintf("GRANT %s TO [%s]", permission.Name, userName)
			if permission.WithGrantOption {
				stat += " WITH GRANT OPTION"
			}

			_, err := conn.ExecContext(ctx, stat)
			utils.AddError(ctx, "Failed to grant permission", err)
		})
}

func (db *database) UpdatePermission(ctx context.Context, id GenericDatabasePrincipalId, permission DatabasePermission) {
	conn := db.connect(ctx)
	userName := db.getUserName(ctx, id)

	utils.StopOnError(ctx).
		Then(func() {
			stat := fmt.Sprintf("GRANT %s TO [%s] WITH GRANT OPTION", permission.Name, userName)
			if !permission.WithGrantOption {
				stat = fmt.Sprintf("REVOKE GRANT OPTION FOR %s TO [%s]", permission.Name, userName)
			}

			_, err := conn.ExecContext(ctx, stat)
			utils.AddError(ctx, "Failed to modify permission grant", err)
		})
}

func (db *database) RevokePermission(ctx context.Context, id GenericDatabasePrincipalId, permissionName string) {
	conn := db.connect(ctx)
	userName := db.getUserName(ctx, id)

	utils.StopOnError(ctx).
		Then(func() {
			stat := fmt.Sprintf("REVOKE %s TO [%s] CASCADE", permissionName, userName)
			_, err := conn.ExecContext(ctx, stat)
			utils.AddError(ctx, "Failed to revoke permission", err)
		})
}

func (db *database) getSettingsRaw(ctx context.Context) (DatabaseSettings, error) {
	var settings DatabaseSettings
	err := db.conn.getSqlConnection(ctx).
		QueryRowContext(ctx, "SELECT [name], collation_name FROM sys.databases WHERE [database_id] = @p1", db.id).
		Scan(&settings.Name, &settings.Collation)
	return settings, err
}

func (db *database) connect(ctx context.Context) *sql.DB {
	settings := db.GetSettings(ctx)
	if utils.HasError(ctx) {
		return nil
	}

	return db.conn.getDBSqlConnection(ctx, settings.Name)
}

func (db *database) getUserName(ctx context.Context, id GenericDatabasePrincipalId) string {
	var (
		name string
		conn *sql.DB
	)

	utils.StopOnError(ctx).
		Then(func() { conn = db.connect(ctx) }).
		Then(func() {
			var err error
			if id == EmptyDatabasePrincipalId {
				err = conn.QueryRowContext(ctx, "SELECT USER_NAME()").Scan(&name)
			} else {
				err = conn.QueryRowContext(ctx, "SELECT USER_NAME(@p1)", id).Scan(&name)
			}

			if err != nil {
				utils.AddError(ctx, "Failed to fetch user name", err)
			}
		})

	return name
}
