package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	dedicatedImp "github.com/tidbcloud/terraform-provider-tidbcloud/pkg/tidbcloud/v1beta1/dedicated/imp"
)

type dedicatedImportResourceData struct {
	ImportId         types.String                 `tfsdk:"import_id"`
	ClusterId        types.String                 `tfsdk:"cluster_id"`
	ImportOptions    *dedicatedImportOptions      `tfsdk:"import_options"`
	Source           *dedicatedImportSource       `tfsdk:"source"`
	TargetTableInfos []dedicatedImportTargetTable `tfsdk:"target_table_infos"`
	State            types.String                 `tfsdk:"state"`
	TotalSize        types.String                 `tfsdk:"total_size"`
	CreateTime       types.String                 `tfsdk:"create_time"`
	CompleteTime     types.String                 `tfsdk:"complete_time"`
	CompletePercent  types.Int32                  `tfsdk:"complete_percent"`
	Message          types.String                 `tfsdk:"message"`
	Creator          types.String                 `tfsdk:"creator"`
}

type dedicatedImportOptions struct {
	FileType            types.String     `tfsdk:"file_type"`
	CsvFormat           *importCsvFormat `tfsdk:"csv_format"`
	DuplicationHandling types.String     `tfsdk:"duplication_handling"`
}

type dedicatedImportSource struct {
	Type      types.String           `tfsdk:"type"`
	S3        *importS3Source        `tfsdk:"s3"`
	Gcs       *importGcsSource       `tfsdk:"gcs"`
	AzureBlob *importAzureBlobSource `tfsdk:"azure_blob"`
}

type dedicatedImportTargetTable struct {
	Schema     types.String `tfsdk:"schema"`
	Table      types.String `tfsdk:"table"`
	CustomFile types.String `tfsdk:"custom_file"`
}

type dedicatedImportResource struct {
	provider *tidbcloudProvider
}

func NewDedicatedImportResource() resource.Resource {
	return &dedicatedImportResource{}
}

func (r *dedicatedImportResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_import"
}

func (r *dedicatedImportResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	var ok bool
	if r.provider, ok = req.ProviderData.(*tidbcloudProvider); !ok {
		resp.Diagnostics.AddError("Internal provider error",
			fmt.Sprintf("Error in Configure: expected %T but got %T", tidbcloudProvider{}, req.ProviderData))
	}
}

func (r *dedicatedImportResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "dedicated import resource. Import data from Amazon S3, Google Cloud Storage or Azure Blob Storage into a TiDB Cloud Dedicated cluster.",
		Attributes: map[string]schema.Attribute{
			"import_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the import.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster to import into.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"import_options": schema.SingleNestedAttribute{
				MarkdownDescription: "The options of the import.",
				Required:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"file_type": schema.StringAttribute{
						MarkdownDescription: "The file type of the import. Available values are CSV, SQL, AURORA_SNAPSHOT and PARQUET.",
						Required:            true,
					},
					"csv_format": schema.SingleNestedAttribute{
						MarkdownDescription: "The CSV format of the import.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"separator": schema.StringAttribute{
								MarkdownDescription: "Separator of each value in CSV files. Default is ','.",
								Optional:            true,
							},
							"delimiter": schema.StringAttribute{
								MarkdownDescription: "Delimiter of string type variables in CSV files. Default is '\"'.",
								Optional:            true,
							},
							"header": schema.BoolAttribute{
								MarkdownDescription: "Import CSV files of the tables with header. Default is true.",
								Optional:            true,
							},
							"not_null": schema.BoolAttribute{
								MarkdownDescription: "Whether the columns in CSV files can be null. Default is false.",
								Optional:            true,
							},
							"null_value": schema.StringAttribute{
								MarkdownDescription: `Representation of null values in CSV files. Default is "\N".`,
								Optional:            true,
							},
							"backslash_escape": schema.BoolAttribute{
								MarkdownDescription: "Whether to escape backslashes in CSV files. Default is true.",
								Optional:            true,
							},
							"trim_last_separator": schema.BoolAttribute{
								MarkdownDescription: "Whether to trim the last separator in CSV files. Default is false.",
								Optional:            true,
							},
						},
					},
					"duplication_handling": schema.StringAttribute{
						MarkdownDescription: "Specifies how to handle duplicate records when importing SQL files. Available values are REPLACE, IGNORE and ERROR.",
						Optional:            true,
					},
				},
			},
			"source": schema.SingleNestedAttribute{
				MarkdownDescription: "The source of the import.",
				Required:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "The import source type. Available values are S3, GCS and AZURE_BLOB.",
						Required:            true,
					},
					"s3": schema.SingleNestedAttribute{
						MarkdownDescription: "The S3 source.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"uri": schema.StringAttribute{
								MarkdownDescription: "The S3 URI of the import source.",
								Required:            true,
							},
							"auth_type": schema.StringAttribute{
								MarkdownDescription: "The auth method of the import source. Available values are ROLE_ARN and ACCESS_KEY.",
								Required:            true,
							},
							"role_arn": schema.StringAttribute{
								MarkdownDescription: "The AWS IAM role ARN used for access. This is required when auth_type is ROLE_ARN.",
								Optional:            true,
							},
							"access_key": schema.SingleNestedAttribute{
								MarkdownDescription: "The access key of the S3. This is required when auth_type is ACCESS_KEY.",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										MarkdownDescription: "The access key ID of the S3.",
										Required:            true,
									},
									"secret": schema.StringAttribute{
										MarkdownDescription: "The secret access key of the S3. This field is input-only.",
										Required:            true,
										Sensitive:           true,
									},
								},
							},
						},
					},
					"gcs": schema.SingleNestedAttribute{
						MarkdownDescription: "The GCS source.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"uri": schema.StringAttribute{
								MarkdownDescription: "The GCS URI of the import source.",
								Required:            true,
							},
							"auth_type": schema.StringAttribute{
								MarkdownDescription: "The auth method of the import source. Available value is SERVICE_ACCOUNT_KEY.",
								Required:            true,
							},
							"service_account_key": schema.StringAttribute{
								MarkdownDescription: "The service account key of the GCS. This field is input-only.",
								Optional:            true,
								Sensitive:           true,
							},
						},
					},
					"azure_blob": schema.SingleNestedAttribute{
						MarkdownDescription: "The Azure Blob source.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"uri": schema.StringAttribute{
								MarkdownDescription: "The Azure Blob URI of the import source. For example: azure://<account>.blob.core.windows.net/<container>/<path> or https://<account>.blob.core.windows.net/<container>/<path>.",
								Required:            true,
							},
							"auth_type": schema.StringAttribute{
								MarkdownDescription: "The auth method of the import source. Available value is SAS_TOKEN.",
								Required:            true,
							},
							"sas_token": schema.StringAttribute{
								MarkdownDescription: "The sas token of the Azure Blob. This field is input-only.",
								Optional:            true,
								Sensitive:           true,
							},
						},
					},
				},
			},
			"target_table_infos": schema.ListNestedAttribute{
				MarkdownDescription: "A list of destination tables and their configurations for the import. If not specified, TiDB Cloud Dedicated uses the default naming conventions to match files to tables.",
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"schema": schema.StringAttribute{
							MarkdownDescription: "The name of the database containing the target table.",
							Optional:            true,
						},
						"table": schema.StringAttribute{
							MarkdownDescription: "The name of the table within the database.",
							Optional:            true,
						},
						"custom_file": schema.StringAttribute{
							MarkdownDescription: "The custom file URI to import data into the target table.",
							Optional:            true,
						},
					},
				},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "The state of the import.",
				Computed:            true,
			},
			"total_size": schema.StringAttribute{
				MarkdownDescription: "The total size of the data to be imported, in bytes.",
				Computed:            true,
			},
			"create_time": schema.StringAttribute{
				MarkdownDescription: "The time the import was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"complete_time": schema.StringAttribute{
				MarkdownDescription: "The time the import was completed.",
				Computed:            true,
			},
			"complete_percent": schema.Int32Attribute{
				MarkdownDescription: "The percentage of import progress, excluding post-processing.",
				Computed:            true,
			},
			"message": schema.StringAttribute{
				MarkdownDescription: "An error message if the import task failed. Otherwise, empty.",
				Computed:            true,
			},
			"creator": schema.StringAttribute{
				MarkdownDescription: "The email address of the user who created the import task.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *dedicatedImportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.provider.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// get data from config
	var data dedicatedImportResourceData
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "create dedicated_import_resource")
	body, err := buildCreateDedicatedImportBody(data)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", fmt.Sprintf("Unable to build CreateImport body, got error: %s", err))
		return
	}

	imp, err := r.provider.DedicatedClient.CreateImport(ctx, data.ClusterId.ValueString(), &body)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", fmt.Sprintf("Unable to create import, got error: %s", err))
		return
	}

	data.ImportId = types.StringValue(*imp.ImportId)
	refreshDedicatedImportResourceData(imp, &data)

	// save to terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *dedicatedImportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dedicatedImportResourceData
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	imp, err := r.provider.DedicatedClient.GetImport(ctx, data.ClusterId.ValueString(), data.ImportId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read import, got error: %s", err))
		return
	}

	refreshDedicatedImportResourceData(imp, &data)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *dedicatedImportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (r *dedicatedImportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var clusterId string
	var importId string

	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("cluster_id"), &clusterId)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("import_id"), &importId)...)
	if resp.Diagnostics.HasError() {
		return
	}

	imp, err := r.provider.DedicatedClient.GetImport(ctx, clusterId, importId)
	if err != nil {
		resp.Diagnostics.AddError("Delete Error", fmt.Sprintf("Unable to get dedicated import, got error: %s", err))
		return
	}
	// there is no delete API for imports, cancel the import if it is still running
	if imp.State != nil && (*imp.State == dedicatedImp.V1BETA1IMPORTSTATEENUM_PREPARING || *imp.State == dedicatedImp.V1BETA1IMPORTSTATEENUM_IMPORTING) {
		tflog.Trace(ctx, "dedicated_import_resource is running, cancel it before delete")
		err := r.provider.DedicatedClient.CancelImport(ctx, clusterId, importId)
		if err != nil {
			resp.Diagnostics.AddError("Cancel Error", fmt.Sprintf("Unable to cancel dedicated import before delete, got error: %s", err))
			return
		}
	}
}

func (r *dedicatedImportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: cluster_id, import_id. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("import_id"), idParts[1])...)
}

func buildCreateDedicatedImportBody(data dedicatedImportResourceData) (dedicatedImp.V1beta1Import, error) {
	body := dedicatedImp.V1beta1Import{}

	// import options
	fileType := dedicatedImp.V1beta1ImportFileTypeEnum(data.ImportOptions.FileType.ValueString())
	importOptions := dedicatedImp.V1beta1ImportOptions{
		FileType: fileType,
	}
	if data.ImportOptions.CsvFormat != nil {
		c := data.ImportOptions.CsvFormat
		csvFormat := dedicatedImp.V1beta1CSVFormat{}
		if IsKnown(c.Separator) {
			separator := c.Separator.ValueString()
			csvFormat.Separator = &separator
		}
		if IsKnown(c.Delimiter) {
			delimiter := c.Delimiter.ValueString()
			csvFormat.Delimiter = &delimiter
		}
		if IsKnown(c.Header) {
			header := c.Header.ValueBool()
			csvFormat.Header = &header
		}
		if IsKnown(c.NotNull) {
			notNull := c.NotNull.ValueBool()
			csvFormat.NotNull = &notNull
		}
		if IsKnown(c.NullValue) {
			nullValue := c.NullValue.ValueString()
			csvFormat.NullValue = &nullValue
		}
		if IsKnown(c.BackslashEscape) {
			backslashEscape := c.BackslashEscape.ValueBool()
			csvFormat.BackslashEscape = &backslashEscape
		}
		if IsKnown(c.TrimLastSeparator) {
			trimLastSeparator := c.TrimLastSeparator.ValueBool()
			csvFormat.TrimLastSeparator = &trimLastSeparator
		}
		importOptions.CsvFormat = &csvFormat
	}
	if IsKnown(data.ImportOptions.DuplicationHandling) {
		duplicationHandling := dedicatedImp.V1beta1DuplicationHandlingForSQLEnum(data.ImportOptions.DuplicationHandling.ValueString())
		importOptions.DuplicationHandling = &duplicationHandling
	}

	// source
	sourceType := dedicatedImp.V1beta1ImportSourceTypeEnum(data.Source.Type.ValueString())
	source := dedicatedImp.V1beta1ImportSource{
		Type: sourceType,
	}
	switch sourceType {
	case dedicatedImp.V1BETA1IMPORTSOURCETYPEENUM_S3:
		if data.Source.S3 == nil {
			return body, fmt.Errorf("source.s3 is required when source type is S3")
		}
		source.S3 = &dedicatedImp.V1beta1S3Source{
			Uri:      data.Source.S3.Uri.ValueString(),
			AuthType: dedicatedImp.V1beta1ImportS3AuthTypeEnum(data.Source.S3.AuthType.ValueString()),
		}
		if IsKnown(data.Source.S3.RoleArn) {
			roleArn := data.Source.S3.RoleArn.ValueString()
			source.S3.RoleArn = &roleArn
		}
		if data.Source.S3.AccessKey != nil {
			source.S3.AccessKey = &dedicatedImp.S3SourceAccessKey{
				Id:     data.Source.S3.AccessKey.Id.ValueString(),
				Secret: data.Source.S3.AccessKey.Secret.ValueString(),
			}
		}
	case dedicatedImp.V1BETA1IMPORTSOURCETYPEENUM_GCS:
		if data.Source.Gcs == nil {
			return body, fmt.Errorf("source.gcs is required when source type is GCS")
		}
		source.Gcs = &dedicatedImp.V1beta1GCSSource{
			Uri:      data.Source.Gcs.Uri.ValueString(),
			AuthType: dedicatedImp.V1beta1ImportGcsAuthTypeEnum(data.Source.Gcs.AuthType.ValueString()),
		}
		if IsKnown(data.Source.Gcs.ServiceAccountKey) {
			serviceAccountKey := data.Source.Gcs.ServiceAccountKey.ValueString()
			source.Gcs.ServiceAccountKey = &serviceAccountKey
		}
	case dedicatedImp.V1BETA1IMPORTSOURCETYPEENUM_AZURE_BLOB:
		if data.Source.AzureBlob == nil {
			return body, fmt.Errorf("source.azure_blob is required when source type is AZURE_BLOB")
		}
		source.AzureBlob = &dedicatedImp.V1beta1AzureBlobSource{
			Uri:      data.Source.AzureBlob.Uri.ValueString(),
			AuthType: dedicatedImp.V1beta1ImportAzureBlobAuthTypeEnum(data.Source.AzureBlob.AuthType.ValueString()),
		}
		if IsKnown(data.Source.AzureBlob.SasToken) {
			sasToken := data.Source.AzureBlob.SasToken.ValueString()
			source.AzureBlob.SasToken = &sasToken
		}
	default:
		return body, fmt.Errorf("unsupported source type %q, available values are S3, GCS and AZURE_BLOB", string(sourceType))
	}

	body.CreationDetails = dedicatedImp.V1beta1CreationDetails{
		ImportOptions: importOptions,
		Source:        source,
	}

	// target table infos
	for _, t := range data.TargetTableInfos {
		info := dedicatedImp.V1beta1ImportTargetTableInfo{}
		if IsKnown(t.Schema) || IsKnown(t.Table) {
			table := dedicatedImp.CommonTable{}
			if IsKnown(t.Schema) {
				schema := t.Schema.ValueString()
				table.Schema = &schema
			}
			if IsKnown(t.Table) {
				tableName := t.Table.ValueString()
				table.Table = &tableName
			}
			info.TargetTable = &table
		}
		if IsKnown(t.CustomFile) {
			customFile := t.CustomFile.ValueString()
			info.CustomFile = &customFile
		}
		body.CreationDetails.TargetTableInfos = append(body.CreationDetails.TargetTableInfos, info)
	}

	return body, nil
}

// refreshDedicatedImportResourceData writes the computed attributes from the API
// response into data. The user-supplied import_options and source are immutable
// (RequiresReplace) and contain input-only secrets that the API never returns, so
// they are only populated from creationDetails when absent (e.g. terraform import).
func refreshDedicatedImportResourceData(resp *dedicatedImp.V1beta1Import, data *dedicatedImportResourceData) {
	data.ImportId = types.StringValue(*resp.ImportId)
	if resp.State != nil {
		data.State = types.StringValue(string(*resp.State))
	}
	if resp.TotalSize != nil {
		data.TotalSize = types.StringValue(*resp.TotalSize)
	}
	if resp.CreateTime != nil {
		data.CreateTime = types.StringValue(resp.CreateTime.Format(time.RFC3339))
	}
	if resp.CompleteTime != nil {
		data.CompleteTime = types.StringValue(resp.CompleteTime.Format(time.RFC3339))
	}
	if resp.CompletePercent != nil {
		data.CompletePercent = types.Int32Value(*resp.CompletePercent)
	}
	if resp.Message != nil {
		data.Message = types.StringValue(*resp.Message)
	}
	if resp.Creator != nil {
		data.Creator = types.StringValue(*resp.Creator)
	}

	// populate the input attributes from creationDetails only when they are not
	// already present, so that `terraform import` produces a usable state without
	// overwriting user input (secrets are input-only and never returned).
	cd := resp.CreationDetails
	if data.ImportOptions == nil {
		io := dedicatedImportOptions{
			FileType: types.StringValue(string(cd.ImportOptions.FileType)),
		}
		if cd.ImportOptions.CsvFormat != nil {
			f := cd.ImportOptions.CsvFormat
			csv := importCsvFormat{}
			if f.Separator != nil {
				csv.Separator = types.StringValue(*f.Separator)
			}
			if f.Delimiter != nil {
				csv.Delimiter = types.StringValue(*f.Delimiter)
			}
			if f.Header != nil {
				csv.Header = types.BoolValue(*f.Header)
			}
			if f.NotNull != nil {
				csv.NotNull = types.BoolValue(*f.NotNull)
			}
			if f.NullValue != nil {
				csv.NullValue = types.StringValue(*f.NullValue)
			}
			if f.BackslashEscape != nil {
				csv.BackslashEscape = types.BoolValue(*f.BackslashEscape)
			}
			if f.TrimLastSeparator != nil {
				csv.TrimLastSeparator = types.BoolValue(*f.TrimLastSeparator)
			}
			io.CsvFormat = &csv
		}
		if cd.ImportOptions.DuplicationHandling != nil {
			io.DuplicationHandling = types.StringValue(string(*cd.ImportOptions.DuplicationHandling))
		}
		data.ImportOptions = &io
	}
	if data.Source == nil {
		s := dedicatedImportSource{
			Type: types.StringValue(string(cd.Source.Type)),
		}
		switch cd.Source.Type {
		case dedicatedImp.V1BETA1IMPORTSOURCETYPEENUM_S3:
			if cd.Source.S3 != nil {
				s3 := importS3Source{
					Uri:      types.StringValue(cd.Source.S3.Uri),
					AuthType: types.StringValue(string(cd.Source.S3.AuthType)),
				}
				if cd.Source.S3.RoleArn != nil {
					s3.RoleArn = types.StringValue(*cd.Source.S3.RoleArn)
				}
				s.S3 = &s3
			}
		case dedicatedImp.V1BETA1IMPORTSOURCETYPEENUM_GCS:
			if cd.Source.Gcs != nil {
				s.Gcs = &importGcsSource{
					Uri:      types.StringValue(cd.Source.Gcs.Uri),
					AuthType: types.StringValue(string(cd.Source.Gcs.AuthType)),
				}
			}
		case dedicatedImp.V1BETA1IMPORTSOURCETYPEENUM_AZURE_BLOB:
			if cd.Source.AzureBlob != nil {
				s.AzureBlob = &importAzureBlobSource{
					Uri:      types.StringValue(cd.Source.AzureBlob.Uri),
					AuthType: types.StringValue(string(cd.Source.AzureBlob.AuthType)),
				}
			}
		}
		data.Source = &s
	}
}
