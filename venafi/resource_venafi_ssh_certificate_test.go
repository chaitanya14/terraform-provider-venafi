package venafi

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"os"
	"strconv"
	"strings"
	"testing"
)

var (
	envSshCertVariables = fmt.Sprintf(`
variable "TPP_USER" {default = "%s"}
variable "TPP_PASSWORD" {default = "%s"}
variable "TPP_URL" {default = "%s"}
variable "TPP_ZONE" {default = "%s"}
variable "TRUST_BUNDLE" {default = "%s"}
variable "TPP_ACCESS_TOKEN" {default = "%s"}
`,
		os.Getenv("TPP_USER"),
		os.Getenv("TPP_PASSWORD"),
		os.Getenv("TPP_URL"),
		os.Getenv("TPP_ZONE"),
		os.Getenv("TRUST_BUNDLE"),
		os.Getenv("TPP_ACCESS_TOKEN"))

	tokenSshCertProv = environmentVariables + `
provider "venafi" {
	url = "${var.TPP_URL}"
	access_token = "${var.TPP_ACCESS_TOKEN}"
	trust_bundle = "${file(var.TRUST_BUNDLE)}"
}`

	tppSshCertResourceTest = `
%s
resource "venafi_ssh_certificate" "test" {
	provider = "venafi"
	key_id="%s"
	template="%s"
	public_key_method="%s"
	valid_hours = %s
	principal=[
		%s
	]
	source_address=[
		"%s"
	]
}

output "certificate"{
	value = venafi_ssh_certificate.test.certificate
}
output "public_key"{
	value = venafi_ssh_certificate.test.public_key
}
output "private_key"{
	value = venafi_ssh_certificate.test.private_key
}
output "principals"{
	value = venafi_ssh_certificate.test.principal
}`
	tppSshCertResourceTestNewAttrPrincipals = `
%s
resource "venafi_ssh_certificate" "test-new-principals" {
	provider = "venafi"
	key_id="%s"
	template="%s"
	public_key_method="%s"
	valid_hours = %s
	principals=[
		%s
	]
	source_address=[
		"%s"
	]
}

output "certificate"{
	value = venafi_ssh_certificate.test-new-principals.certificate
}
output "public_key"{
	value = venafi_ssh_certificate.test-new-principals.public_key
}
output "private_key"{
	value = venafi_ssh_certificate.test-new-principals.private_key
}
output "principals"{
	value = venafi_ssh_certificate.test-new-principals.principals
}`
)

func TestSshCert(t *testing.T) {

	t.Log("Testing creating ssh certificate")

	data := getTestData()
	data.publicKeyMethod = "service"

	// data.principals only holds the values for principals, actually we are testing "principal" attribute defined at the resource.
	config := fmt.Sprintf(tppSshCertResourceTest, tokenSshCertProv, data.keyId, data.template, data.publicKeyMethod, data.validityPeriod, data.principals, data.sourceAddress)
	t.Logf("Testing SSH certificate with config:\n %s", config)
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					checkSshCertificate("venafi_ssh_certificate.test", t, &data),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestSshCertNewAttrPrincipals(t *testing.T) {
	t.Log("Testing creating ssh certificate with new attribute for principals")

	data := getTestData()
	data.publicKeyMethod = "service"

	config := fmt.Sprintf(tppSshCertResourceTestNewAttrPrincipals, tokenSshCertProv, data.keyId, data.template, data.publicKeyMethod, data.validityPeriod, data.principals, data.sourceAddress)
	t.Logf("Testing SSH certificate with config:\n %s", config)
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					checkSshCertificate("venafi_ssh_certificate.test-new-principals", t, &data),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestSshCertLocalPublicKey(t *testing.T) {
	t.Log("Testing creating ssh certificate")

	data := getTestData()
	data.publicKeyMethod = "local"

	// data.principals only holds the values for principals, actually we are testing "principal" attribute defined at the resource.
	config := fmt.Sprintf(tppSshCertResourceTest, tokenSshCertProv, data.keyId, data.template, data.publicKeyMethod, data.validityPeriod, data.principals, data.sourceAddress)
	t.Logf("Testing SSH certificate with config:\n %s", config)
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					checkSshCertificate("venafi_ssh_certificate.test", t, &data),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestSshCertLocalPublicKeyNewAttrPrincipals(t *testing.T) {
	t.Log("Testing creating ssh certificate with new attribute for principals")

	data := getTestData()
	data.publicKeyMethod = "local"

	config := fmt.Sprintf(tppSshCertResourceTestNewAttrPrincipals, tokenSshCertProv, data.keyId, data.template, data.publicKeyMethod, data.validityPeriod, data.principals, data.sourceAddress)
	t.Logf("Testing SSH certificate with config:\n %s", config)
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					checkSshCertificate("venafi_ssh_certificate.test-new-principals", t, &data),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func checkSshCertificate(resourceName string, t *testing.T, data *testData) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		t.Log("Testing SSH certificate with key-id", data.keyId)
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}
		certificate := rs.Primary.Attributes["certificate"]
		if certificate == "" {
			return fmt.Errorf("certificate is empty")
		}
		privateKey := rs.Primary.Attributes["private_key"]
		if privateKey == "" {
			return fmt.Errorf("private key is empty")
		}

		publicKey := rs.Primary.Attributes["public_key"]
		if publicKey == "" {
			return fmt.Errorf("certificate is empty")
		}

		principalsLengthString := rs.Primary.Attributes["principals.#"]
		principalsLength, err := strconv.Atoi(principalsLengthString)
		if err != nil {
			fmt.Errorf("error getting length: %s", err)
		}
		if principalsLength <= 0 {
			principalLengthString := rs.Primary.Attributes["principal.#"]
			principalLength, err := strconv.Atoi(principalLengthString)
			if err != nil {
				fmt.Errorf("error getting length: %s", err)
			}
			if principalLength <= 0 && data.principals != "" {
				fmt.Errorf("principal list is empty")
			}
		}
		return nil
	}
}

func getTestData() testData {
	return testData{
		keyId:          RandTppSshCertName(),
		template:       os.Getenv("TPP_SSH_CA"),
		sourceAddress:  "test.com",
		validityPeriod: "4",
		principals:     "\"" + strings.Join([]string{"bob", "alice"}, `", "`) + "\"",
	}
}
