package sql

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/suite"
)

func TestSqlLoginTestSuite(t *testing.T) {
	s := &SqlLoginTestSuite{}
	suite.Run(t, s)
}

type SqlLoginTestSuite struct {
	SqlTestSuite
	login sqlLogin
}

func (s *SqlLoginTestSuite) SetupTest() {
	s.SqlTestSuite.SetupTest()
	s.login = sqlLogin{conn: s.connMock, id: LoginId(fmt.Sprint(rand.Int()))}
}

func (s *SqlLoginTestSuite) TestGetSqlLoginByName() {
	const loginId = "0x12541251"
	s.expectSqlLoginIdQuery("test_login").WillReturnRows(newRows("ID").AddRow(loginId))

	login := GetSqlLoginByName(s.ctx, s.connMock, "test_login")

	s.Equal(LoginId(loginId), login.GetId(s.ctx), "DB ID")
}

func (s *SqlLoginTestSuite) TestGetSqlLoginByNameError() {
	err := errors.New("test DB error")
	s.expectSqlLoginIdQuery("test_login").WillReturnError(err)

	login := GetSqlLoginByName(s.ctx, s.connMock, "test_login")

	s.Nil(login, "login")
	s.verifyError(err)
}

func (s *SqlLoginTestSuite) TestGetMultipleSqlLogins() {
	loginIds := []LoginId{"0x13412", "0x653573"}
	rows := newRows("id")
	for _, id := range loginIds {
		rows.AddRow(id)
	}
	s.expectSqlLoginsQuery().WillReturnRows(rows)

	logins := GetSqlLogins(s.ctx, s.connMock)

	s.Equal(2, len(logins), "Logins count")
	for _, expectedId := range loginIds {
		login, ok := logins[expectedId]
		s.True(ok, "Login with ID %s not found", expectedId)
		s.Equal(expectedId, login.GetId(s.ctx), "Login instance points to invalid ID")
	}
}

func (s *SqlLoginTestSuite) TestGetLoginsNoRows() {
	s.expectSqlLoginsQuery().WillReturnError(sql.ErrNoRows)

	s.Equal(0, len(GetSqlLogins(s.ctx, s.connMock)), "Expected empty logins slice")
}

func (s *SqlLoginTestSuite) TestGetLoginsError() {
	err := errors.New("test_error")
	s.expectSqlLoginsQuery().WillReturnError(err)

	GetSqlLogins(s.ctx, s.connMock)

	s.verifyError(err)
}

func (s *SqlLoginTestSuite) TestCreateSqlLogin() {
	cases := map[string]struct {
		settings SqlLoginSettings
		edition  SQLEdition
		sql      string
	}{
		"simple login": {
			settings: SqlLoginSettings{Name: "simple", Password: "simple_password"},
			edition:  EDITION_ENTERPRISE,
			sql:      "CREATE LOGIN [simple] WITH PASSWORD='simple_password', CHECK_EXPIRATION=OFF, CHECK_POLICY=OFF",
		},
		"must change": {
			settings: SqlLoginSettings{Name: "must_change", Password: "must change password", CheckPasswordExpiration: true, MustChangePassword: true},
			edition:  EDITION_ENTERPRISE,
			sql:      "CREATE LOGIN [must_change] WITH PASSWORD='must change password' MUST_CHANGE, CHECK_EXPIRATION=ON, CHECK_POLICY=OFF",
		},
		"default language": {
			settings: SqlLoginSettings{Name: "default_language", Password: "test_password", CheckPasswordPolicy: true, DefaultLanguage: "test_language"},
			edition:  EDITION_ENTERPRISE,
			sql:      "CREATE LOGIN [default_language] WITH PASSWORD='test_password', DEFAULT_LANGUAGE=[test_language], CHECK_EXPIRATION=OFF, CHECK_POLICY=ON",
		},
		"login with SID": {
			settings: SqlLoginSettings{Name: "login_with_sid", Password: "test_password", Id: "0xE0EEDFAEC6DD7443BBFDE477C0D1E8A1"},
			edition:  EDITION_ENTERPRISE,
			sql:      "CREATE LOGIN [login_with_sid] WITH PASSWORD='test_password', SID=0xE0EEDFAEC6DD7443BBFDE477C0D1E8A1, CHECK_EXPIRATION=OFF, CHECK_POLICY=OFF",
		},
		"Azure SQL": {
			settings: SqlLoginSettings{
				Name:                    "default_language",
				Password:                "test_password",
				CheckPasswordPolicy:     true,
				DefaultLanguage:         "test_language",
				MustChangePassword:      true,
				DefaultDatabaseId:       13,
				CheckPasswordExpiration: true,
			},
			edition: EDITION_AZURE_SQL,
			sql:     "CREATE LOGIN [default_language] WITH PASSWORD='test_password'",
		},
	}

	for name, t := range cases {
		tc := t
		s.Run(name, func() {
			const id = "0x1362311"
			s.expectEditionQuery(tc.edition)
			expectExactExec(s.mock, tc.sql).WillReturnResult(sqlmock.NewResult(0, 1))
			s.expectSqlLoginIdQuery(tc.settings.Name).WillReturnRows(newRows("id").AddRow(id))

			login := CreateSqlLogin(s.ctx, s.connMock, tc.settings)

			s.Require().NotNil(login)
			s.Equal(LoginId(id), login.GetId(s.ctx), "Login ID")
		})
	}
}

func (s *SqlLoginTestSuite) TestCreateSqlLoginDefaultDb() {
	const id = "0x4746854"
	settings := SqlLoginSettings{Name: "test_login", Password: "test_password", DefaultDatabaseId: DatabaseId(1324)}
	s.expectEditionQuery(EDITION_ENTERPRISE)
	expectExactQuery(s.mock, "SELECT DB_NAME(@p1)").WithArgs(settings.DefaultDatabaseId).WillReturnRows(newRows("name").AddRow("test_db"))
	expectExactExec(s.mock, "CREATE LOGIN [test_login] WITH PASSWORD='test_password', DEFAULT_DATABASE=[test_db], CHECK_EXPIRATION=OFF, CHECK_POLICY=OFF").
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.expectSqlLoginIdQuery("test_login").WillReturnRows(newRows("id").AddRow(id))

	login := CreateSqlLogin(s.ctx, s.connMock, settings)

	s.Require().NotNil(login)
	s.Equal(LoginId(id), login.GetId(s.ctx), "Login ID")
}

func (s *SqlLoginTestSuite) TestCreateSqlLoginError() {
	settings := SqlLoginSettings{Name: "test_login", Password: "test_password"}
	err := errors.New("test_error")
	s.expectEditionQuery(EDITION_ENTERPRISE)
	expectExactExec(s.mock, "CREATE LOGIN [test_login] WITH PASSWORD='test_password', CHECK_EXPIRATION=OFF, CHECK_POLICY=OFF").WillReturnError(err)

	login := CreateSqlLogin(s.ctx, s.connMock, settings)

	s.Nil(login, "login")
	s.verifyError(err)
}

func (s *SqlLoginTestSuite) TestExistsMissing() {
	s.expectSqlLoginNamesByIdQuery().WithArgs(s.login.id).WillReturnError(sql.ErrNoRows)

	s.False(s.login.Exists(s.ctx))
}

func (s *SqlLoginTestSuite) TestExists() {
	s.expectSqlLoginNamesByIdQuery().WithArgs(s.login.id).WillReturnRows(newRows("name").AddRow("name"))

	s.True(s.login.Exists(s.ctx))
}

func (s *SqlLoginTestSuite) TestExistsError() {
	err := errors.New("test_error")
	s.expectSqlLoginNamesByIdQuery().WithArgs(s.login.id).WillReturnError(err)

	s.login.Exists(s.ctx)

	s.verifyError(err)
}

func (s *SqlLoginTestSuite) TestGetSettings() {
	expectedSettings := SqlLoginSettings{
		Name:                    "test_name",
		Password:                "test_hash",
		MustChangePassword:      true,
		DefaultDatabaseId:       134,
		DefaultLanguage:         "test_lang",
		CheckPasswordExpiration: true,
		CheckPasswordPolicy:     false,
		PrincipalId:             235,
	}
	rows := newRows("name", "password_hash", "is_must_change", "default_database_id", "default_language_name", "is_expiration_checked", "is_policy_checked", "principal_id").
		AddRow(expectedSettings.Name, expectedSettings.Password, 1, expectedSettings.DefaultDatabaseId, expectedSettings.DefaultLanguage, 1, 0, 235)
	s.expectSettingsQuery().WithArgs(s.login.id).WillReturnRows(rows)

	settings := s.login.GetSettings(s.ctx)

	s.Equal(expectedSettings, settings)
}

func (s *SqlLoginTestSuite) TestGetSettingsWithoutPasswordHashPerms() {
	expectedSettings := SqlLoginSettings{
		Name:                    "test_name",
		Password:                "",
		MustChangePassword:      true,
		DefaultDatabaseId:       134,
		DefaultLanguage:         "test_lang",
		CheckPasswordExpiration: true,
		CheckPasswordPolicy:     false,
		PrincipalId:             46,
	}
	rows := newRows("name", "password_hash", "is_must_change", "default_database_id", "default_language_name", "is_expiration_checked", "is_policy_checked", "principal_id").
		AddRow(expectedSettings.Name, nil, 1, expectedSettings.DefaultDatabaseId, expectedSettings.DefaultLanguage, 1, 0, 46)
	s.expectSettingsQuery().WithArgs(s.login.id).WillReturnRows(rows)

	settings := s.login.GetSettings(s.ctx)

	s.Equal(expectedSettings, settings)
}

func (s *SqlLoginTestSuite) TestGetSettingsAzureSql() {
	expectedSettings := SqlLoginSettings{
		Name:                    "test_name",
		Password:                "test_hash",
		MustChangePassword:      false,
		DefaultDatabaseId:       0,
		DefaultLanguage:         "",
		CheckPasswordExpiration: false,
		CheckPasswordPolicy:     false,
		PrincipalId:             68,
	}
	rows := newRows("name", "password_hash", "is_must_change", "default_database_id", "default_language_name", "is_expiration_checked", "is_policy_checked", "principal_id").
		AddRow(expectedSettings.Name, expectedSettings.Password, nil, expectedSettings.DefaultDatabaseId, expectedSettings.DefaultLanguage, 0, 0, 68)
	s.expectSettingsQuery().WithArgs(s.login.id).WillReturnRows(rows)

	settings := s.login.GetSettings(s.ctx)

	s.Equal(expectedSettings, settings)
}

func (s *SqlLoginTestSuite) TestGetSettingsError() {
	err := errors.New("test_error")
	s.expectSettingsQuery().WithArgs(s.login.id).WillReturnError(err)

	s.login.GetSettings(s.ctx)

	s.verifyError(err)
}

func (s *SqlLoginTestSuite) TestUpdateSqlLoginSettings() {
	cases := map[string]struct {
		settings SqlLoginSettings
		edition  SQLEdition
		sql      string
	}{
		"simple login": {
			settings: SqlLoginSettings{Name: "simple", Password: "simple_password"},
			edition:  EDITION_ENTERPRISE,
			sql:      "ALTER LOGIN [old_name] WITH PASSWORD='simple_password', CHECK_EXPIRATION=OFF, CHECK_POLICY=OFF, NAME=[simple]",
		},
		"must change": {
			settings: SqlLoginSettings{Name: "must_change", Password: "must change password", CheckPasswordExpiration: true, MustChangePassword: true},
			edition:  EDITION_ENTERPRISE,
			sql:      "ALTER LOGIN [old_name] WITH PASSWORD='must change password' MUST_CHANGE, CHECK_EXPIRATION=ON, CHECK_POLICY=OFF, NAME=[must_change]",
		},
		"default language": {
			settings: SqlLoginSettings{Name: "default_language", Password: "test_password", CheckPasswordPolicy: true, DefaultLanguage: "test_language"},
			edition:  EDITION_ENTERPRISE,
			sql:      "ALTER LOGIN [old_name] WITH PASSWORD='test_password', DEFAULT_LANGUAGE=[test_language], CHECK_EXPIRATION=OFF, CHECK_POLICY=ON, NAME=[default_language]",
		},
		"Azure SQL": {
			settings: SqlLoginSettings{
				Name:                    "azure_sql",
				Password:                "test_password",
				CheckPasswordExpiration: true,
				MustChangePassword:      true,
				CheckPasswordPolicy:     true,
				DefaultDatabaseId:       1343,
				DefaultLanguage:         "us_english",
			},
			edition: EDITION_AZURE_SQL,
			sql:     "ALTER LOGIN [old_name] WITH PASSWORD='test_password', NAME=[azure_sql]",
		},
	}

	for name, tc := range cases {
		s.Run(name, func() {
			s.expectEditionQuery(tc.edition)
			s.expectSqlLoginNameLookupQuery().WithArgs(s.login.id).WillReturnRows(newRows("name").AddRow("old_name"))
			expectExactExec(s.mock, tc.sql).WillReturnResult(sqlmock.NewResult(0, 1))

			s.login.UpdateSettings(s.ctx, tc.settings)
		})
	}
}

func (s *SqlLoginTestSuite) TestUpdateSqlLoginSettingsDefaultDb() {
	settings := SqlLoginSettings{Name: "test_login", Password: "test_password", DefaultDatabaseId: DatabaseId(1324)}
	s.expectEditionQuery(EDITION_ENTERPRISE)
	expectExactQuery(s.mock, "SELECT DB_NAME(@p1)").WithArgs(settings.DefaultDatabaseId).WillReturnRows(newRows("name").AddRow("test_db"))
	s.expectSqlLoginNameLookupQuery().WithArgs(s.login.id).WillReturnRows(newRows("name").AddRow("old_name"))
	expectExactExec(s.mock, "ALTER LOGIN [old_name] WITH PASSWORD='test_password', DEFAULT_DATABASE=[test_db], CHECK_EXPIRATION=OFF, CHECK_POLICY=OFF, NAME=[test_login]").
		WillReturnResult(sqlmock.NewResult(0, 1))

	s.login.UpdateSettings(s.ctx, settings)
}

func (s *SqlLoginTestSuite) TestUpdateSqlLoginSettingsError() {
	err := errors.New("test_error")
	settings := SqlLoginSettings{Name: "invalid_login", Password: "test_password"}
	s.expectEditionQuery(EDITION_ENTERPRISE)
	s.expectSqlLoginNameLookupQuery().WithArgs(s.login.id).WillReturnRows(newRows("name").AddRow(settings.Name))
	expectExactExec(s.mock, "ALTER LOGIN [invalid_login] WITH PASSWORD='test_password', CHECK_EXPIRATION=OFF, CHECK_POLICY=OFF, NAME=[invalid_login]").
		WillReturnError(err)

	s.login.UpdateSettings(s.ctx, settings)

	s.verifyError(err)
}

func (s *SqlLoginTestSuite) TestDropSqlLogin() {
	s.expectSqlLoginNameLookupQuery().WithArgs(s.login.id).WillReturnRows(newRows("name").AddRow("test_login"))
	expectExactExec(s.mock, "DROP LOGIN [test_login]").WillReturnResult(sqlmock.NewResult(0, 1))

	s.login.Drop(s.ctx)
}

func (s *SqlLoginTestSuite) expectSqlLoginsQuery() *sqlmock.ExpectedQuery {
	return expectExactQuery(s.mock, "SELECT CONVERT(VARCHAR(85), [sid], 1) FROM sys.sql_logins")
}

func (s *SqlLoginTestSuite) expectSqlLoginIdQuery(loginName string) *sqlmock.ExpectedQuery {
	return expectExactQuery(s.mock, "SELECT CONVERT(VARCHAR(85), [sid], 1) FROM sys.sql_logins WHERE [name]=@p1").WithArgs(loginName)
}

func (s *SqlLoginTestSuite) expectSqlLoginNamesByIdQuery() *sqlmock.ExpectedQuery {
	return expectExactQuery(s.mock, "SELECT [name] FROM sys.sql_logins WHERE CONVERT(VARCHAR(85), [sid], 1) = @p1")
}

func (s *SqlLoginTestSuite) expectSettingsQuery() *sqlmock.ExpectedQuery {
	return expectExactQuery(s.mock, `SELECT 
    l.[name], 
    l.password_hash, 
    LOGINPROPERTY(l.[name], 'IsMustChange') AS is_must_change, 
    db.database_id AS default_database_id, 
    l.default_language_name, 
    l.is_expiration_checked, 
    l.is_policy_checked,
	l.principal_id
FROM sys.sql_logins AS l
INNER JOIN sys.databases AS db ON l.default_database_name = db.[name]
WHERE CONVERT(VARCHAR(85), l.[sid], 1) = @p1`)
}
