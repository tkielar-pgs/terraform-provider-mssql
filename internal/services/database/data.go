package database

import (
	"context"
	"fmt"
	"github.com/PGSSoft/terraform-provider-mssql/internal/core/datasource"
	"github.com/PGSSoft/terraform-provider-mssql/internal/services/common"
	"github.com/PGSSoft/terraform-provider-mssql/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/PGSSoft/terraform-provider-mssql/internal/sql"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dataSource struct{}

func (d *dataSource) GetName() string {
	return "database"
}

func (d *dataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema.MarkdownDescription = "Obtains information about single database."
	resp.Schema.Attributes = map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: attrDescriptions["id"],
			Computed:            true,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: attrDescriptions["name"],
			Required:            true,
		},
		"collation": schema.StringAttribute{
			MarkdownDescription: attrDescriptions["collation"],
			Computed:            true,
		},
		"force_drop": schema.BoolAttribute{
			MarkdownDescription: attrDescriptions["force_drop"],
			Computed:            true,
		},		
	}
}

func (d *dataSource) Read(ctx context.Context, req datasource.ReadRequest[resourceData], resp *datasource.ReadResponse[resourceData]) {
	var db sql.Database

	req.
		Then(func() {
			db = sql.GetDatabaseByName(ctx, req.Conn, req.Config.Name.ValueString())

			if db == nil || !db.Exists(ctx) {
				utils.AddError(ctx, "DB does not exist", fmt.Errorf("could not find DB '%s'", req.Config.Name.ValueString()))
			}
		}).
		Then(func() {
			state := req.Config.withSettings(db.GetSettings(ctx))

			if !common.IsAttrSet(state.Id) {
				state.Id = types.StringValue(fmt.Sprint(db.GetId(ctx)))
			}

			resp.SetState(state)
		})
}
