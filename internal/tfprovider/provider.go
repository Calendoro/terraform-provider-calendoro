// Package tfprovider exposes the generated resource table to Terraform.
package tfprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	calendoro "github.com/Calendoro/calendoro-go"
	"github.com/Calendoro/terraform-provider-calendoro/internal/crud"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type calendoroProvider struct{}

// New builds the provider.
func New() provider.Provider { return &calendoroProvider{} }

func (p *calendoroProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "calendoro"
}

func (p *calendoroProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = pschema.Schema{Attributes: map[string]pschema.Attribute{
		"api_key":  pschema.StringAttribute{Optional: true, Sensitive: true, Description: "API key. Defaults to CALENDORO_API_KEY. An OAuth access token also works."},
		"base_url": pschema.StringAttribute{Optional: true, Description: "API URL. Defaults to CALENDORO_BASE_URL, then https://api.calendoro.app."},
	}}
}

func (p *calendoroProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg struct {
		APIKey  types.String `tfsdk:"api_key"`
		BaseURL types.String `tfsdk:"base_url"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := cfg.APIKey.ValueString()
	if key == "" {
		key = os.Getenv("CALENDORO_API_KEY")
	}
	if key == "" {
		resp.Diagnostics.AddError("Missing API key", "Set api_key on the provider or the CALENDORO_API_KEY environment variable.")
		return
	}
	base := cfg.BaseURL.ValueString()
	if base == "" {
		base = os.Getenv("CALENDORO_BASE_URL")
	}
	opts := []calendoro.Option{calendoro.WithAPIKey(key)}
	if base != "" {
		opts = append(opts, calendoro.WithBaseURL(base))
	}
	api := &crud.API{C: calendoro.New(opts...).Client}
	resp.ResourceData = api
}

func (p *calendoroProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func (p *calendoroProvider) Resources(context.Context) []func() resource.Resource {
	out := make([]func() resource.Resource, 0, len(crud.Resources))
	for _, d := range crud.Resources {
		out = append(out, func() resource.Resource { return &genericResource{def: d} })
	}
	return out
}

// genericResource is one row of the table.
type genericResource struct {
	def crud.ResourceDef
	api *crud.API
}

var (
	_ resource.ResourceWithConfigure   = &genericResource{}
	_ resource.ResourceWithImportState = &genericResource{}
)

func (r *genericResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.def.Type
}

func attrSchema(a crud.AttrDef) schema.Attribute {
	optional := !a.Required
	switch a.Kind {
	case crud.KindInt:
		pm := []planmodifier.Int64{}
		if a.ForceNew {
			pm = append(pm, int64planmodifier.RequiresReplace())
		}
		if optional {
			pm = append(pm, int64planmodifier.UseStateForUnknown())
		}
		return schema.Int64Attribute{Required: a.Required, Optional: optional, Computed: optional, Description: a.Doc, PlanModifiers: pm}
	case crud.KindFloat:
		pm := []planmodifier.Float64{}
		if a.ForceNew {
			pm = append(pm, float64planmodifier.RequiresReplace())
		}
		if optional {
			pm = append(pm, float64planmodifier.UseStateForUnknown())
		}
		return schema.Float64Attribute{Required: a.Required, Optional: optional, Computed: optional, Description: a.Doc, PlanModifiers: pm}
	case crud.KindBool:
		pm := []planmodifier.Bool{}
		if a.ForceNew {
			pm = append(pm, boolplanmodifier.RequiresReplace())
		}
		if optional {
			pm = append(pm, boolplanmodifier.UseStateForUnknown())
		}
		return schema.BoolAttribute{Required: a.Required, Optional: optional, Computed: optional, Description: a.Doc, PlanModifiers: pm}
	default: // strings and JSON text
		pm := []planmodifier.String{}
		if a.ForceNew {
			pm = append(pm, stringplanmodifier.RequiresReplace())
		}
		if optional {
			pm = append(pm, stringplanmodifier.UseStateForUnknown())
		}
		return schema.StringAttribute{Required: a.Required, Optional: optional, Computed: optional, Description: a.Doc, PlanModifiers: pm}
	}
}

func (r *genericResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
	}
	for _, a := range r.def.Attrs {
		attrs[a.Name] = attrSchema(a)
	}
	if r.def.Secret != "" {
		attrs["secret"] = schema.StringAttribute{Computed: true, Sensitive: true, Description: "Signing secret. The API shows it only at creation, so it is empty after an import.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
	}
	resp.Schema = schema.Schema{Description: "Calendoro " + r.def.Type, Attributes: attrs}
}

func (r *genericResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*crud.API)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("%T", req.ProviderData))
		return
	}
	r.api = api
}

type getter func(context.Context, path.Path, any) diag.Diagnostics

// planned reads an attribute. Known is false when it is null or unknown, which leaves it out of requests: an explicit null cannot clear a field here.
func planned(ctx context.Context, get getter, a crud.AttrDef, diags *diag.Diagnostics) (value any, known bool, val attr.Value) {
	p := path.Root(a.Name)
	switch a.Kind {
	case crud.KindInt:
		var v types.Int64
		diags.Append(get(ctx, p, &v)...)
		return v.ValueInt64(), !v.IsNull() && !v.IsUnknown(), v
	case crud.KindFloat:
		var v types.Float64
		diags.Append(get(ctx, p, &v)...)
		return v.ValueFloat64(), !v.IsNull() && !v.IsUnknown(), v
	case crud.KindBool:
		var v types.Bool
		diags.Append(get(ctx, p, &v)...)
		return v.ValueBool(), !v.IsNull() && !v.IsUnknown(), v
	default:
		var v types.String
		diags.Append(get(ctx, p, &v)...)
		ok := !v.IsNull() && !v.IsUnknown()
		if a.Kind == crud.KindJSON && ok {
			var parsed any
			if err := json.Unmarshal([]byte(v.ValueString()), &parsed); err != nil {
				diags.AddAttributeError(p, "Invalid JSON", err.Error())
				return nil, false, v
			}
			return parsed, true, v
		}
		return v.ValueString(), ok, v
	}
}

// remoteValue turns a response value into the attribute's Terraform value.
func remoteValue(a crud.AttrDef, v any, prior string, diags *diag.Diagnostics) attr.Value {
	switch a.Kind {
	case crud.KindInt:
		if n, ok := v.(float64); ok {
			return types.Int64Value(int64(n))
		}
		return types.Int64Null()
	case crud.KindFloat:
		if n, ok := v.(float64); ok {
			return types.Float64Value(n)
		}
		return types.Float64Null()
	case crud.KindBool:
		if b, ok := v.(bool); ok {
			return types.BoolValue(b)
		}
		return types.BoolNull()
	case crud.KindJSON:
		if v == nil {
			return types.StringNull()
		}
		text, err := crud.ReconcileJSON(prior, v)
		if err != nil {
			diags.AddError("Could not encode "+a.Name, err.Error())
			return types.StringNull()
		}
		return types.StringValue(text)
	default:
		if s, ok := v.(string); ok {
			return types.StringValue(s)
		}
		return types.StringNull()
	}
}

// applyResult writes state after a create or update. Values the plan fixed stay exactly as planned, or Terraform rejects the result; the next read reports real drift.
func (r *genericResource) applyResult(ctx context.Context, state setter, get getter, obj map[string]any, diags *diag.Diagnostics) {
	for _, a := range r.def.Attrs {
		_, known, val := planned(ctx, get, a, diags)
		if known {
			diags.Append(state(ctx, path.Root(a.Name), val)...)
			continue
		}
		diags.Append(state(ctx, path.Root(a.Name), remoteValue(a, obj[a.Wire], "", diags))...)
	}
}

type setter func(context.Context, path.Path, any) diag.Diagnostics

func (r *genericResource) body(ctx context.Context, get getter, forUpdate bool, diags *diag.Diagnostics) map[string]any {
	body := map[string]any{}
	for _, a := range r.def.Attrs {
		if forUpdate && a.ForceNew {
			continue
		}
		if v, known, _ := planned(ctx, get, a, diags); known {
			body[a.Wire] = v
		}
	}
	return body
}

func (r *genericResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	body := r.body(ctx, req.Plan.GetAttribute, false, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, secret, err := r.api.Create(ctx, r.def, body)
	if err != nil {
		resp.Diagnostics.AddError("Create failed", err.Error())
		return
	}
	id, _ := obj["id"].(string)
	resp.State.Raw = req.Plan.Raw
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if r.def.Secret != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("secret"), secret)...)
	}
	r.applyResult(ctx, resp.State.SetAttribute, req.Plan.GetAttribute, obj, &resp.Diagnostics)
}

func (r *genericResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.api.Get(ctx, r.def, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read failed", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	for _, a := range r.def.Attrs {
		v, present := obj[a.Wire]
		if !present {
			continue // write-only fields, such as webhook custom headers, are never returned
		}
		prior := ""
		if a.Kind == crud.KindJSON {
			var s types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(a.Name), &s)...)
			prior = s.ValueString()
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(a.Name), remoteValue(a, v, prior, &resp.Diagnostics))...)
	}
}

func (r *genericResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	body := r.body(ctx, req.Plan.GetAttribute, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.api.Update(ctx, r.def, id.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Update failed", err.Error())
		return
	}
	resp.State.Raw = req.Plan.Raw
	r.applyResult(ctx, resp.State.SetAttribute, req.Plan.GetAttribute, obj, &resp.Diagnostics)
}

func (r *genericResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.api.Delete(ctx, r.def, id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete failed", err.Error())
	}
}

func (r *genericResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
