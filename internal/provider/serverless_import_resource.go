package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	impV1beta1 "github.com/tidbcloud/tidbcloud-cli/pkg/tidbcloud/v1beta1/serverless/imp"
)

type serverlessImportResourceData struct {
	ImportId        types.String   `tfsdk:"import_id"`
	ClusterId       types.String   `tfsdk:"cluster_id"`
	ImportOptions   *importOptions `tfsdk:"import_options"`
	Source          *importSource  `tfsdk:"source"`
	State           types.String   `tfsdk:"state"`
	TotalSize       types.String   `tfsdk:"total_size"`
	CreateTime      types.String   `tfsdk:"create_time"`
	CompleteTime    types.String   `tfsdk:"complete_time"`
	CompletePercent types.Int64    `tfsdk:"complete_percent"`
	Message         types.String   `tfsdk:"message"`
	CreatedBy       types.String   `tfsdk:"created_by"`
}

type importOptions struct {
	FileType  types.String     `tfsdk:"file_type"`
	CsvFormat *importCsvFormat `tfsdk:"csv_format"`
}

type importCsvFormat struct {
	Separator         types.String `tfsdk:"separator"`
	Delimiter         types.String `tfsdk:"delimiter"`
	Header            types.Bool   `tfsdk:"header"`
	NotNull           types.Bool   `tfsdk:"not_null"`
	NullValue         types.String `tfsdk:"null_value"`
	BackslashEscape   types.Bool   `tfsdk:"backslash_escape"`
	TrimLastSeparator types.Bool   `tfsdk:"trim_last_separator"`
}

type importSource struct {
	Type         types.String              `tfsdk:"type"`
	S3           *importS3Source           `tfsdk:"s3"`
	Gcs          *importGcsSource          `tfsdk:"gcs"`
	AzureBlob    *importAzureBlobSource    `tfsdk:"azure_blob"`
	S3Compatible *importS3CompatibleSource `tfsdk:"s3_compatible"`
	Oss          *importOssSource          `tfsdk:"oss"`
}

type importS3Source struct {
	Uri       types.String `tfsdk:"uri"`
	AuthType  types.String `tfsdk:"auth_type"`
	RoleArn   types.String `tfsdk:"role_arn"`
	AccessKey *accessKey   `tfsdk:"access_key"`
}

type importGcsSource struct {
	Uri               types.String `tfsdk:"uri"`
	AuthType          types.String `tfsdk:"auth_type"`
	ServiceAccountKey types.String `tfsdk:"service_account_key"`
}

type importAzureBlobSource struct {
	Uri      types.String `tfsdk:"uri"`
	AuthType types.String `tfsdk:"auth_type"`
	SasToken types.String `tfsdk:"sas_token"`
}

type importS3CompatibleSource struct {
	Uri       types.String `tfsdk:"uri"`
	AuthType  types.String `tfsdk:"auth_type"`
	AccessKey *accessKey   `tfsdk:"access_key"`
	Endpoint  types.String `tfsdk:"endpoint"`
}

type importOssSource struct {
	Uri       types.String `tfsdk:"uri"`
	AuthType  types.String `tfsdk:"auth_type"`
	AccessKey *accessKey   `tfsdk:"access_key"`
}

type serverlessImportResource struct {
	provider *tidbcloudProvider
}

func NewServerlessImportResource() resource.Resource {
	return &serverlessImportResource{}
}

func (r *serverlessImportResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_serverless_import"
}

func (r *serverlessImportResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverlessImportResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "serverless import resource. Import data from Amazon S3, Google Cloud Storage, Azure Blob Storage, Alibaba Cloud OSS or S3-compatible storage into a TiDB Cloud Serverless cluster.",
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
						MarkdownDescription: "The import source type. Available values are S3, GCS, AZURE_BLOB, S3_COMPATIBLE and OSS.",
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
								MarkdownDescription: "The role arn of the S3.",
								Optional:            true,
							},
							"access_key": schema.SingleNestedAttribute{
								MarkdownDescription: "The access key of the S3.",
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
					"s3_compatible": schema.SingleNestedAttribute{
						MarkdownDescription: "The S3 compatible source.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"uri": schema.StringAttribute{
								MarkdownDescription: "The S3 compatible URI of the import source.",
								Required:            true,
							},
							"auth_type": schema.StringAttribute{
								MarkdownDescription: "The auth method of the import source. Available value is ACCESS_KEY.",
								Required:            true,
							},
							"access_key": schema.SingleNestedAttribute{
								MarkdownDescription: "The access key of the S3 compatible storage.",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										MarkdownDescription: "The access key ID of the S3 compatible storage.",
										Required:            true,
									},
									"secret": schema.StringAttribute{
										MarkdownDescription: "The secret access key of the S3 compatible storage. This field is input-only.",
										Required:            true,
										Sensitive:           true,
									},
								},
							},
							"endpoint": schema.StringAttribute{
								MarkdownDescription: "The custom S3 endpoint (HTTPS only). Used for connecting to non-AWS S3-compatible storage.",
								Optional:            true,
							},
						},
					},
					"oss": schema.SingleNestedAttribute{
						MarkdownDescription: "The Alibaba Cloud OSS source.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"uri": schema.StringAttribute{
								MarkdownDescription: "The OSS URI of the import source.",
								Required:            true,
							},
							"auth_type": schema.StringAttribute{
								MarkdownDescription: "The auth method of the import source. Available value is ACCESS_KEY.",
								Required:            true,
							},
							"access_key": schema.SingleNestedAttribute{
								MarkdownDescription: "The access key of the OSS.",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										MarkdownDescription: "The access key ID of the OSS.",
										Required:            true,
									},
									"secret": schema.StringAttribute{
										MarkdownDescription: "The secret access key of the OSS. This field is input-only.",
										Required:            true,
										Sensitive:           true,
									},
								},
							},
						},
					},
				},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "The state of the import.",
				Computed:            true,
			},
			"total_size": schema.StringAttribute{
				MarkdownDescription: "The total size of the data imported.",
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
			"complete_percent": schema.Int64Attribute{
				MarkdownDescription: "The process in percent of the import job, but doesn't include the post-processing progress.",
				Computed:            true,
			},
			"message": schema.StringAttribute{
				MarkdownDescription: "The output message of the import.",
				Computed:            true,
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "The user who created the import.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *serverlessImportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.provider.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// get data from config
	var data serverlessImportResourceData
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "create serverless_import_resource")
	body, err := buildCreateServerlessImportBody(data)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", fmt.Sprintf("Unable to build CreateImport body, got error: %s", err))
		return
	}

	imp, err := r.provider.ServerlessClient.CreateImport(ctx, data.ClusterId.ValueString(), &body)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", fmt.Sprintf("Unable to create import, got error: %s", err))
		return
	}

	data.ImportId = types.StringValue(*imp.ImportId)
	refreshServerlessImportResourceData(imp, &data)

	// save to terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *serverlessImportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data serverlessImportResourceData
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	imp, err := r.provider.ServerlessClient.GetImport(ctx, data.ClusterId.ValueString(), data.ImportId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read import, got error: %s", err))
		return
	}

	refreshServerlessImportResourceData(imp, &data)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *serverlessImportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (r *serverlessImportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var clusterId string
	var importId string

	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("cluster_id"), &clusterId)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("import_id"), &importId)...)
	if resp.Diagnostics.HasError() {
		return
	}

	imp, err := r.provider.ServerlessClient.GetImport(ctx, clusterId, importId)
	if err != nil {
		resp.Diagnostics.AddError("Delete Error", fmt.Sprintf("Unable to get serverless import, got error: %s", err))
		return
	}
	// there is no delete API for imports, cancel the import if it is still running
	if imp.State != nil && (*imp.State == impV1beta1.IMPORTSTATEENUM_PREPARING || *imp.State == impV1beta1.IMPORTSTATEENUM_IMPORTING) {
		tflog.Trace(ctx, "serverless_import_resource is running, cancel it before delete")
		err := r.provider.ServerlessClient.CancelImport(ctx, clusterId, importId)
		if err != nil {
			resp.Diagnostics.AddError("Cancel Error", fmt.Sprintf("Unable to cancel serverless import before delete, got error: %s", err))
			return
		}
	}
}

func (r *serverlessImportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

func buildCreateServerlessImportBody(data serverlessImportResourceData) (impV1beta1.ImportServiceCreateImportBody, error) {
	body := impV1beta1.ImportServiceCreateImportBody{}

	// import options
	fileType := impV1beta1.ImportFileTypeEnum(data.ImportOptions.FileType.ValueString())
	body.ImportOptions = impV1beta1.ImportOptions{
		FileType: fileType,
	}
	if data.ImportOptions.CsvFormat != nil {
		csvFormat := impV1beta1.NewCSVFormat()
		c := data.ImportOptions.CsvFormat
		if IsKnown(c.Separator) {
			separator := c.Separator.ValueString()
			csvFormat.Separator = &separator
		}
		if IsKnown(c.Delimiter) {
			delimiter := c.Delimiter.ValueString()
			csvFormat.Delimiter = *impV1beta1.NewNullableString(&delimiter)
		}
		if IsKnown(c.Header) {
			header := c.Header.ValueBool()
			csvFormat.Header = *impV1beta1.NewNullableBool(&header)
		}
		if IsKnown(c.NotNull) {
			notNull := c.NotNull.ValueBool()
			csvFormat.NotNull = *impV1beta1.NewNullableBool(&notNull)
		}
		if IsKnown(c.NullValue) {
			nullValue := c.NullValue.ValueString()
			csvFormat.Null = *impV1beta1.NewNullableString(&nullValue)
		}
		if IsKnown(c.BackslashEscape) {
			backslashEscape := c.BackslashEscape.ValueBool()
			csvFormat.BackslashEscape = *impV1beta1.NewNullableBool(&backslashEscape)
		}
		if IsKnown(c.TrimLastSeparator) {
			trimLastSeparator := c.TrimLastSeparator.ValueBool()
			csvFormat.TrimLastSeparator = *impV1beta1.NewNullableBool(&trimLastSeparator)
		}
		body.ImportOptions.CsvFormat = csvFormat
	}

	// source
	sourceType := impV1beta1.ImportSourceTypeEnum(data.Source.Type.ValueString())
	if sourceType == impV1beta1.IMPORTSOURCETYPEENUM_LOCAL {
		return body, fmt.Errorf("LOCAL source is not supported by this resource, use cloud storage (S3, GCS, AZURE_BLOB, S3_COMPATIBLE or OSS) instead")
	}
	body.Source = impV1beta1.ImportSource{
		Type: sourceType,
	}
	switch sourceType {
	case impV1beta1.IMPORTSOURCETYPEENUM_S3:
		if data.Source.S3 == nil {
			return body, fmt.Errorf("source.s3 is required when source type is S3")
		}
		authType := impV1beta1.ImportS3AuthTypeEnum(data.Source.S3.AuthType.ValueString())
		body.Source.S3 = &impV1beta1.S3Source{
			Uri:      data.Source.S3.Uri.ValueString(),
			AuthType: authType,
		}
		if IsKnown(data.Source.S3.RoleArn) {
			roleArn := data.Source.S3.RoleArn.ValueString()
			body.Source.S3.RoleArn = &roleArn
		}
		if data.Source.S3.AccessKey != nil {
			body.Source.S3.AccessKey = &impV1beta1.S3SourceAccessKey{
				Id:     data.Source.S3.AccessKey.Id.ValueString(),
				Secret: data.Source.S3.AccessKey.Secret.ValueString(),
			}
		}
	case impV1beta1.IMPORTSOURCETYPEENUM_GCS:
		if data.Source.Gcs == nil {
			return body, fmt.Errorf("source.gcs is required when source type is GCS")
		}
		authType := impV1beta1.ImportGcsAuthTypeEnum(data.Source.Gcs.AuthType.ValueString())
		body.Source.Gcs = &impV1beta1.GCSSource{
			Uri:      data.Source.Gcs.Uri.ValueString(),
			AuthType: authType,
		}
		if IsKnown(data.Source.Gcs.ServiceAccountKey) {
			serviceAccountKey := data.Source.Gcs.ServiceAccountKey.ValueString()
			body.Source.Gcs.ServiceAccountKey = &serviceAccountKey
		}
	case impV1beta1.IMPORTSOURCETYPEENUM_AZURE_BLOB:
		if data.Source.AzureBlob == nil {
			return body, fmt.Errorf("source.azure_blob is required when source type is AZURE_BLOB")
		}
		authType := impV1beta1.ImportAzureBlobAuthTypeEnum(data.Source.AzureBlob.AuthType.ValueString())
		body.Source.AzureBlob = &impV1beta1.AzureBlobSource{
			Uri:      data.Source.AzureBlob.Uri.ValueString(),
			AuthType: authType,
		}
		if IsKnown(data.Source.AzureBlob.SasToken) {
			sasToken := data.Source.AzureBlob.SasToken.ValueString()
			body.Source.AzureBlob.SasToken = &sasToken
		}
	case impV1beta1.IMPORTSOURCETYPEENUM_S3_COMPATIBLE:
		if data.Source.S3Compatible == nil {
			return body, fmt.Errorf("source.s3_compatible is required when source type is S3_COMPATIBLE")
		}
		authType := impV1beta1.ImportS3CompatibleAuthTypeEnum(data.Source.S3Compatible.AuthType.ValueString())
		body.Source.S3Compatible = &impV1beta1.S3CompatibleSource{
			Uri:      data.Source.S3Compatible.Uri.ValueString(),
			AuthType: authType,
		}
		if data.Source.S3Compatible.AccessKey != nil {
			body.Source.S3Compatible.AccessKey = &impV1beta1.S3CompatibleSourceAccessKey{
				Id:     data.Source.S3Compatible.AccessKey.Id.ValueString(),
				Secret: data.Source.S3Compatible.AccessKey.Secret.ValueString(),
			}
		}
		if IsKnown(data.Source.S3Compatible.Endpoint) {
			endpoint := data.Source.S3Compatible.Endpoint.ValueString()
			body.Source.S3Compatible.Endpoint = *impV1beta1.NewNullableString(&endpoint)
		}
	case impV1beta1.IMPORTSOURCETYPEENUM_OSS:
		if data.Source.Oss == nil {
			return body, fmt.Errorf("source.oss is required when source type is OSS")
		}
		authType := impV1beta1.ImportOSSAuthTypeEnum(data.Source.Oss.AuthType.ValueString())
		body.Source.Oss = &impV1beta1.OSSSource{
			Uri:      data.Source.Oss.Uri.ValueString(),
			AuthType: authType,
		}
		if data.Source.Oss.AccessKey != nil {
			body.Source.Oss.AccessKey = &impV1beta1.OSSSourceAccessKey{
				Id:     data.Source.Oss.AccessKey.Id.ValueString(),
				Secret: data.Source.Oss.AccessKey.Secret.ValueString(),
			}
		}
	}

	return body, nil
}

// refreshServerlessImportResourceData writes the computed attributes from the API
// response into data. The user-supplied import_options and source are immutable
// (RequiresReplace) and contain input-only secrets that the API never returns, so
// they are only populated from creationDetails when absent (e.g. terraform import).
func refreshServerlessImportResourceData(resp *impV1beta1.Import, data *serverlessImportResourceData) {
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
	if resp.CompleteTime.IsSet() && resp.CompleteTime.Get() != nil {
		data.CompleteTime = types.StringValue(resp.CompleteTime.Get().Format(time.RFC3339))
	}
	if resp.CompletePercent != nil {
		data.CompletePercent = types.Int64Value(*resp.CompletePercent)
	}
	if resp.Message != nil {
		data.Message = types.StringValue(*resp.Message)
	}
	if resp.CreatedBy != nil {
		data.CreatedBy = types.StringValue(*resp.CreatedBy)
	}

	// populate the input attributes from creationDetails only when they are not
	// already present, so that `terraform import` produces a usable state without
	// overwriting user input (secrets are input-only and never returned).
	if resp.CreationDetails != nil {
		cd := resp.CreationDetails
		if data.ImportOptions == nil && cd.ImportOptions != nil {
			io := importOptions{
				FileType: types.StringValue(string(cd.ImportOptions.FileType)),
			}
			if cd.ImportOptions.CsvFormat != nil {
				f := cd.ImportOptions.CsvFormat
				csv := importCsvFormat{}
				if f.Separator != nil {
					csv.Separator = types.StringValue(*f.Separator)
				}
				if f.Delimiter.IsSet() && f.Delimiter.Get() != nil {
					csv.Delimiter = types.StringValue(*f.Delimiter.Get())
				}
				if f.Header.IsSet() && f.Header.Get() != nil {
					csv.Header = types.BoolValue(*f.Header.Get())
				}
				if f.NotNull.IsSet() && f.NotNull.Get() != nil {
					csv.NotNull = types.BoolValue(*f.NotNull.Get())
				}
				if f.Null.IsSet() && f.Null.Get() != nil {
					csv.NullValue = types.StringValue(*f.Null.Get())
				}
				if f.BackslashEscape.IsSet() && f.BackslashEscape.Get() != nil {
					csv.BackslashEscape = types.BoolValue(*f.BackslashEscape.Get())
				}
				if f.TrimLastSeparator.IsSet() && f.TrimLastSeparator.Get() != nil {
					csv.TrimLastSeparator = types.BoolValue(*f.TrimLastSeparator.Get())
				}
				io.CsvFormat = &csv
			}
			data.ImportOptions = &io
		}
		if data.Source == nil && cd.Source != nil {
			s := importSource{
				Type: types.StringValue(string(cd.Source.Type)),
			}
			switch cd.Source.Type {
			case impV1beta1.IMPORTSOURCETYPEENUM_S3:
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
			case impV1beta1.IMPORTSOURCETYPEENUM_GCS:
				if cd.Source.Gcs != nil {
					s.Gcs = &importGcsSource{
						Uri:      types.StringValue(cd.Source.Gcs.Uri),
						AuthType: types.StringValue(string(cd.Source.Gcs.AuthType)),
					}
				}
			case impV1beta1.IMPORTSOURCETYPEENUM_AZURE_BLOB:
				if cd.Source.AzureBlob != nil {
					s.AzureBlob = &importAzureBlobSource{
						Uri:      types.StringValue(cd.Source.AzureBlob.Uri),
						AuthType: types.StringValue(string(cd.Source.AzureBlob.AuthType)),
					}
				}
			case impV1beta1.IMPORTSOURCETYPEENUM_S3_COMPATIBLE:
				if cd.Source.S3Compatible != nil {
					sc := importS3CompatibleSource{
						Uri:      types.StringValue(cd.Source.S3Compatible.Uri),
						AuthType: types.StringValue(string(cd.Source.S3Compatible.AuthType)),
					}
					if cd.Source.S3Compatible.Endpoint.IsSet() && cd.Source.S3Compatible.Endpoint.Get() != nil {
						sc.Endpoint = types.StringValue(*cd.Source.S3Compatible.Endpoint.Get())
					}
					s.S3Compatible = &sc
				}
			case impV1beta1.IMPORTSOURCETYPEENUM_OSS:
				if cd.Source.Oss != nil {
					s.Oss = &importOssSource{
						Uri:      types.StringValue(cd.Source.Oss.Uri),
						AuthType: types.StringValue(string(cd.Source.Oss.AuthType)),
					}
				}
			}
			data.Source = &s
		}
	}
}
