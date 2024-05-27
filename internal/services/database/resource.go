package database

import (
	"context"
	"fmt"
	"github.com/PGSSoft/terraform-provider-mssql/internal/core/resource"
	"github.com/PGSSoft/terraform-provider-mssql/internal/sql"
	"github.com/PGSSoft/terraform-provider-mssql/internal/validators"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"sync"
)

var resLock sync.Mutex

type res struct{}

func (r *res) GetName() string {
	return "database"
}

func (r *res) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema.MarkdownDescription = "Manages single database."
	resp.Schema.Attributes = map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: attrDescriptions["id"],
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"name": schema.StringAttribute{
			MarkdownDescription: attrDescriptions["name"],
			Required:            true,
			Validators:          validators.DatabaseNameValidators,
		},
		"collation": schema.StringAttribute{
			MarkdownDescription: attrDescriptions["collation"] + " Defaults to SQL Server instance's default collation.",
			Optional:            true,
			Computed:            true,
		},
		"force_drop": schema.BoolAttribute{
			MarkdownDescription: attrDescriptions["force_drop"],
			Computed:            true,
			Optional:            true,
		},	
	}
}

func (r *res) Create(ctx context.Context, req resource.CreateRequest[resourceData], resp *resource.CreateResponse[resourceData]) {
	resLock.Lock()
	defer resLock.Unlock()

	var db sql.Database

	req.
		Then(func() { db = sql.CreateDatabase(ctx, req.Conn, req.Plan.toSettings()) }).
		Then(func() { resp.State = req.Plan.withSettings(db.GetSettings(ctx)) }).
		Then(func() { resp.State.Id = types.StringValue(fmt.Sprint(db.GetId(ctx))) })
}

func (r *res) Read(ctx context.Context, req resource.ReadRequest[resourceData], resp *resource.ReadResponse[resourceData]) {
	var db sql.Database
	var dbExists bool
	var settings sql.DatabaseSettings

	req.
		Then(func() { db = sql.GetDatabase(ctx, req.Conn, req.State.getDbId(ctx)) }).
		Then(func() {
			dbExists = db.Exists(ctx)
			settings = db.GetSettings(ctx)
		}).
		Then(func() {
			if dbExists {
				resp.SetState(req.State.withSettings(settings))
			}
		})
}

func (r *res) Update(ctx context.Context, req resource.UpdateRequest[resourceData], resp *resource.UpdateResponse[resourceData]) {
	resLock.Lock()
	defer resLock.Unlock()

	var db sql.Database

	req.
		Then(func() { db = sql.GetDatabase(ctx, req.Conn, req.Plan.getDbId(ctx)) }).
		Then(func() {
			if req.State.Name.ValueString() != req.Plan.Name.ValueString() {
				db.Rename(ctx, req.Plan.Name.ValueString())
			}
		}).
		Then(func() {
			if req.State.Collation.ValueString() != req.Plan.Collation.ValueString() {
				db.SetCollation(ctx, req.Plan.Collation.ValueString())
			}
		}).
		Then(func() {
			resp.State = req.Plan.withSettings(db.GetSettings(ctx))
		})
}

func (r *res) Delete(ctx context.Context, req resource.DeleteRequest[resourceData], _ *resource.DeleteResponse[resourceData]) {
	resLock.Lock()
	defer resLock.Unlock()

	var dbId sql.DatabaseId
	var db sql.Database

	req.
		Then(func() { dbId = req.State.getDbId(ctx) }).
		Then(func() { db = sql.GetDatabase(ctx, req.Conn, dbId) }).
		Then(func() { db.Drop(ctx) })
}
