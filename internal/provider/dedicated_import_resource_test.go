package provider

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	mockClient "github.com/tidbcloud/terraform-provider-tidbcloud/mock"
	"github.com/tidbcloud/terraform-provider-tidbcloud/tidbcloud"
	dedicatedImp "github.com/tidbcloud/terraform-provider-tidbcloud/pkg/tidbcloud/v1beta1/dedicated/imp"
)

func TestAccDedicatedImportResource(t *testing.T) {
	dedicatedImportResourceName := "tidbcloud_dedicated_import.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccDedicatedImportResourceConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dedicatedImportResourceName, "import_options.file_type", "CSV"),
					resource.TestCheckResourceAttr(dedicatedImportResourceName, "source.type", "S3"),
					resource.TestCheckResourceAttrSet(dedicatedImportResourceName, "import_id"),
					resource.TestCheckResourceAttrSet(dedicatedImportResourceName, "state"),
				),
			},
		},
	})
}

func TestUTDedicatedImportResource(t *testing.T) {
	setupTestEnv()

	ctrl := gomock.NewController(t)
	s := mockClient.NewMockTiDBCloudDedicatedClient(ctrl)
	defer HookGlobal(&NewDedicatedClient, func(publicKey string, privateKey string, dedicatedEndpoint string, userAgent string) (tidbcloud.TiDBCloudDedicatedClient, error) {
		return s, nil
	})()

	importId := "import-id"

	createImportResp := dedicatedImp.V1beta1Import{}
	createImportResp.UnmarshalJSON([]byte(testUTDedicatedImport(string(dedicatedImp.V1BETA1IMPORTSTATEENUM_IMPORTING))))
	getImportResp := dedicatedImp.V1beta1Import{}
	getImportResp.UnmarshalJSON([]byte(testUTDedicatedImport(string(dedicatedImp.V1BETA1IMPORTSTATEENUM_COMPLETED))))

	s.EXPECT().CreateImport(gomock.Any(), gomock.Any(), gomock.Any()).Return(&createImportResp, nil)
	s.EXPECT().GetImport(gomock.Any(), gomock.Any(), importId).Return(&getImportResp, nil).AnyTimes()

	testDedicatedImportResource(t)
}

func TestUTDedicatedImportResourceCancelOnDelete(t *testing.T) {
	setupTestEnv()

	ctrl := gomock.NewController(t)
	s := mockClient.NewMockTiDBCloudDedicatedClient(ctrl)
	defer HookGlobal(&NewDedicatedClient, func(publicKey string, privateKey string, dedicatedEndpoint string, userAgent string) (tidbcloud.TiDBCloudDedicatedClient, error) {
		return s, nil
	})()

	importId := "import-id"

	importingResp := dedicatedImp.V1beta1Import{}
	importingResp.UnmarshalJSON([]byte(testUTDedicatedImport(string(dedicatedImp.V1BETA1IMPORTSTATEENUM_IMPORTING))))

	s.EXPECT().CreateImport(gomock.Any(), gomock.Any(), gomock.Any()).Return(&importingResp, nil)
	s.EXPECT().GetImport(gomock.Any(), gomock.Any(), importId).Return(&importingResp, nil).AnyTimes()
	// the import is still running when the resource is destroyed, so it must be canceled
	s.EXPECT().CancelImport(gomock.Any(), gomock.Any(), importId).Return(nil)

	testDedicatedImportResource(t)
}

func testDedicatedImportResource(t *testing.T) {
	dedicatedImportResourceName := "tidbcloud_dedicated_import.test"
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read dedicated import resource
			{
				Config: testUTDedicatedImportResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dedicatedImportResourceName, "import_id", "import-id"),
					resource.TestCheckResourceAttr(dedicatedImportResourceName, "import_options.file_type", "CSV"),
					resource.TestCheckResourceAttr(dedicatedImportResourceName, "source.type", "S3"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccDedicatedImportResourceConfig() string {
	return `
resource "tidbcloud_dedicated_import" "test" {
	cluster_id = "your-dedicated-cluster-id"
	import_options = {
		file_type = "CSV"
	}
	source = {
		type = "S3"
		s3 = {
			uri       = "s3://test-bucket/test-path/"
			auth_type = "ROLE_ARN"
			role_arn  = "arn:aws:iam::123456789012:role/test-role"
		}
	}
}
`
}

func testUTDedicatedImportResourceConfig() string {
	return `
resource "tidbcloud_dedicated_import" "test" {
	cluster_id = "cluster_id"
	import_options = {
		file_type = "CSV"
	}
	source = {
		type = "S3"
		s3 = {
			uri       = "s3://test-bucket/test-path/"
			auth_type = "ROLE_ARN"
			role_arn  = "arn:aws:iam::123456789012:role/test-role"
		}
	}
}
`
}

func testUTDedicatedImport(state string) string {
	return fmt.Sprintf(`
{
    "importId": "import-id",
    "name": "clusters/cluster-id/imports/import-id",
    "clusterId": "cluster-id",
    "totalSize": "100",
    "createTime": "2025-03-20T05:53:57.000Z",
    "state": "%s",
    "completePercent": 100,
    "message": "",
    "creator": "user@example.com",
    "creationDetails": {
        "importOptions": {
            "fileType": "CSV"
        },
        "source": {
            "type": "S3",
            "s3": {
                "uri": "s3://test-bucket/test-path/",
                "authType": "ROLE_ARN",
                "roleArn": "arn:aws:iam::123456789012:role/test-role"
            }
        }
    }
}
`, state)
}
