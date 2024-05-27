package database

import (
	"context"
	"fmt"
	"github.com/PGSSoft/terraform-provider-mssql/internal/core/datasource"
	"github.com/PGSSoft/terraform-provider-mssql/internal/sql"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type listDataSourceData struct {
	Id        types.String   `tfsdk:"id"`
	Databases []resourceData `tfsdk:"databases"`
}

type listDataSource struct{}

func (l *listDataSource) GetName() string {
	return "databases"
}

func (l *listDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema.MarkdownDescription = "Obtains information about all databases found in SQL Server instance."
	resp.Schema.Attributes = map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description: "ID of the resource used only internally by the provider.",
			Computed:    true,
		},
		"databases": schema.SetNestedAttribute{
			Description: "Set of database objects",
			Computed:    true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						MarkdownDescription: attrDescriptions["id"],
						Computed:            true,
					},
					"name": schema.StringAttribute{
						MarkdownDescription: attrDescriptions["name"],
						Computed:            true,
					},
					"collation": schema.StringAttribute{
						MarkdownDescription: attrDescriptions["collation"],
						Computed:            true,
					},
					"force_drop": schema.BoolAttribute{
						MarkdownDescription: attrDescriptions["force_drop"],
						Computed:            true,
					},						
				},
			},
		},
	}
}

func (l *listDataSource) Read(ctx context.Context, req datasource.ReadRequest[listDataSourceData], resp *datasource.ReadResponse[listDataSourceData]) {
	var dbs map[sql.DatabaseId]sql.Database

	req.
		Then(func() { dbs = sql.GetDatabases(ctx, req.Conn) }).
		Then(func() {
			result := listDataSourceData{
				Id:        types.StringValue(""),
				Databases: []resourceData{},
			}

			for id, db := range dbs {
				r := resourceData{
					Id: types.StringValue(fmt.Sprint(id)),
				}
				result.Databases = append(result.Databases, r.withSettings(db.GetSettings(ctx)))
			}

			resp.SetState(result)
		})
}
