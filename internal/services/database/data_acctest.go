package database

import (
	"fmt"
	"github.com/PGSSoft/terraform-provider-mssql/internal/acctest"
	"github.com/PGSSoft/terraform-provider-mssql/internal/sql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"regexp"
)

func testDataSource(testCtx *acctest.TestContext) {
	const resourceName = "data.mssql_database.test"
	var dbId string
	dbSettings := sql.DatabaseSettings{Name: "data_test_db", Collation: "SQL_Latin1_General_CP1_CS_AS", ForceDrop: false}

	newDataResource := func(name string) string {
		return fmt.Sprintf(`
data "mssql_database" "test" {
	name = %q
}`, name)
	}

	testCtx.Test(resource.TestCase{
		PreCheck: func() {
			dbId = fmt.Sprint(testCtx.CreateDB(dbSettings.Name))
			_, err := testCtx.GetDBConnection(dbSettings.Name).Exec("ALTER DATABASE [data_test_db] COLLATE SQL_Latin1_General_CP1_CS_AS")
			testCtx.Require.NoError(err, "Setting DB collation")
		},
		Steps: []resource.TestStep{
			{
				Config:      newDataResource("not_exists"),
				ExpectError: regexp.MustCompile("not exist"),
			},
			{
				Config: newDataResource("data_test_db"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(resourceName, "id", &dbId),
					resource.TestCheckResourceAttr(resourceName, "name", dbSettings.Name),
					resource.TestCheckResourceAttr(resourceName, "collation", dbSettings.Collation),
				),
			},
		},
	})
}
