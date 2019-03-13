package venafi

import (
	"crypto/tls"
	"fmt"
	"github.com/Venafi/vcert/pkg/endpoint"
	"net"
	"time"

	"github.com/Venafi/vcert"
	"github.com/Venafi/vcert/pkg/certificate"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strings"
	"encoding/pem"
	"crypto/x509"
)

func resourceVenafiCertificate() *schema.Resource {
	return &schema.Resource{
		Create: resourceVenafiCertificateCreate,
		Read:   resourceVenafiCertificateRead,
		Delete: resourceVenafiCertificateDelete,

		Schema: map[string]*schema.Schema{
			"common_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"algorithm": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Default:     "RSA",
				Description: "RSA or ECDSA. RSA is default.",
			},
			"rsa_bits": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Number of bits to use when generating an RSA key",
				ForceNew:    true,
				Default:     2048,
			},

			"ecdsa_curve": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ECDSA curve to use when generating a key",
				ForceNew:    true,
				Default:     "P224",
			},

			"san_dns": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"san_email": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"san_ip": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"key_password": &schema.Schema{
				Type:      schema.TypeString,
				Optional:  true,
				ForceNew:  true,
				Sensitive: true,
			},

			"private_key_pem": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"chain": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"certificate": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"organizational_unit": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"organization_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"country": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"state": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"locality": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"csr_pem": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"certificate_dn": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"expiration_window": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     168,
				Description: "Number of hours before the certificates expiry when a new certificate will be generated",
				ForceNew:    true,
			},
		},
	}
}

func resourceVenafiCertificateCreate(d *schema.ResourceData, meta interface{}) error {
	log.Printf("Creating certificate\n")
	//venafi := meta.(*VenafiClient)
	cfg := meta.(*vcert.Config)
	cl, err := vcert.NewClient(cfg)
	if err != nil {
		log.Printf(messageVenafiClientInitFailed + err.Error())
		return err
	}
	err = cl.Ping()
	if err != nil {
		log.Printf(messageVenafiPingFailed + err.Error())
		return err
	}
	log.Println(messageVenafiPingSucessfull)

	err = enrollVenafiCertificate(d, cl)
	if err != nil {
		return err
	}
	return nil
}

func resourceVenafiCertificateRead(d *schema.ResourceData, meta interface{}) error {

	if certUntyped, ok := d.GetOk("certificate"); ok {
		certPEM := certUntyped.(string)
		block, _ := pem.Decode([]byte(certPEM))
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing cert: %s", err)
		}
		//Checking Private Key
		var pk []byte
		if pkUntyped, ok := d.GetOk("private_key_pem"); ok {
			pk, err = getPrivateKey([]byte(pkUntyped.(string)), d.Get("key_password").(string))
			if err != nil {
				return fmt.Errorf("error getting key: %s", err)
			}
		} else {
			return fmt.Errorf("error getting key")
		}
		_, err = tls.X509KeyPair([]byte(certPEM), pk)
		if err != nil {
			return fmt.Errorf("error comparing certificate and key: %s", err)
		}

		//TODO: maybe this check should be up on CSR creation
		renewWindow := time.Duration(d.Get("expiration_window").(int)) * time.Hour
		certDuration := cert.NotAfter.Sub(cert.NotBefore)
		if certDuration < renewWindow {
			return fmt.Errorf("certificate validity duration %s is less than configured expiration window %s", certDuration, renewWindow)
		}
		durationUntilExpiry := cert.NotAfter.Sub(time.Now())
		renewIn := durationUntilExpiry - renewWindow
		if renewIn <= 0 {
			//TODO: get request id from resource id
			log.Printf("Requesting new certificate because it's expiration date %s is less then exipration window %s", durationUntilExpiry, renewWindow)
			cfg := meta.(*vcert.Config)
			cl, err := vcert.NewClient(cfg)
			if err != nil {
				log.Printf(messageVenafiClientInitFailed + err.Error())
				return err
			}
			err = cl.Ping()
			if err != nil {
				log.Printf(messageVenafiPingFailed + err.Error())
				return err
			}
			log.Println(messageVenafiPingSucessfull)

			err = enrollVenafiCertificate(d, cl)
			if err != nil {
				return err
			}
			return nil
		}


	}

	return nil
}

func resourceVenafiCertificateDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}


func enrollVenafiCertificate(d *schema.ResourceData, cl endpoint.Connector) error {

	req := &certificate.Request{
		CsrOrigin: certificate.LocalGeneratedCSR,
	}

	//Configuring keys
	var (
		err         error
		keyPassword string
	)

	keyType := d.Get("algorithm").(string)

	if pass, ok := d.GetOk("key_password"); ok {
		keyPassword = pass.(string)
		req.KeyPassword = keyPassword
	}

	if keyType == "RSA" || len(keyType) == 0 {
		req.KeyLength = d.Get("rsa_bits").(int)
		req.KeyType = certificate.KeyTypeRSA
	} else if keyType == "ECDSA" {
		keyCurve := d.Get("ecdsa_curve").(string)
		req.KeyType = certificate.KeyTypeECDSA
		switch {
		case keyCurve == "P224":
			req.KeyCurve = certificate.EllipticCurveP224
		case keyCurve == "P256":
			req.KeyCurve = certificate.EllipticCurveP256
		case keyCurve == "P384":
			req.KeyCurve = certificate.EllipticCurveP384
		case keyCurve == "P521":
			req.KeyCurve = certificate.EllipticCurveP521
		}

	} else {
		return fmt.Errorf("Can't determine key algorithm %s", keyType)
	}

	//Setting up Subject
	commonName := d.Get("common_name").(string)
	//Adding alt names if exists
	dnsnum := d.Get("san_dns.#").(int)
	if dnsnum > 0 {
		for i := 0; i < dnsnum; i++ {
			key := fmt.Sprintf("san_dns.%d", i)
			val := d.Get(key).(string)
			log.Printf("Adding SAN %s.", val)
			req.DNSNames = append(req.DNSNames, val)
		}
	}

	if len(commonName) == 0 && len(req.DNSNames) == 0 {
		return fmt.Errorf("no domains specified on certificate")
	}
	if len(commonName) == 0 && len(req.DNSNames) > 0 {
		commonName = req.DNSNames[0]
	}
	if !sliceContains(req.DNSNames, commonName) {
		log.Printf("Adding CN %s to SAN %s because it wasn't included.", commonName, req.DNSNames)
		req.DNSNames = append(req.DNSNames, commonName)
	}

	//Obtain a certificate from the Venafi server
	log.Printf("Using CN %s and SAN %s", commonName, req.DNSNames)
	req.Subject.CommonName = commonName

	emailnum := d.Get("san_email.#").(int)
	if emailnum > 0 {
		for i := 0; i < emailnum; i++ {
			key := fmt.Sprintf("san_email.%d", i)
			val := d.Get(key).(string)
			req.EmailAddresses = append(req.EmailAddresses, val)
		}
	}
	ipnum := d.Get("san_ip.#").(int)
	if ipnum > 0 {
		ipList := make([]string, 0, ipnum)
		for i := 0; i < ipnum; i++ {
			key := fmt.Sprintf("san_ip.%d", i)
			val := d.Get(key).(string)
			ipList = append(ipList, val)
		}
		for i := 0; i < len(ipList); i += 1 {
			ip := net.ParseIP(ipList[i])
			if ip == nil {
				return fmt.Errorf("invalid IP address %#v", ipList[i])
			}
			req.IPAddresses = append(req.IPAddresses, ip)
		}
	}

	//Appending common name to the DNS names if it is not there
	if !sliceContains(req.DNSNames, commonName) {
		log.Printf("Adding CN %s to SAN because it wasn't included.", commonName)
		req.DNSNames = append(req.DNSNames, commonName)
	}

	log.Printf("Requested SAN: %s", req.DNSNames)

	switch req.KeyType {
	case certificate.KeyTypeECDSA:
		req.PrivateKey, err = certificate.GenerateECDSAPrivateKey(req.KeyCurve)
	case certificate.KeyTypeRSA:
		req.PrivateKey, err = certificate.GenerateRSAPrivateKey(req.KeyLength)
	default:
		return fmt.Errorf("Unable to generate certificate request, key type %s is not supported", req.KeyType.String())
	}

	if err != nil {
		return fmt.Errorf("error generating key: %s", err)
	}

	log.Println("Making certificate request")
	err = cl.GenerateRequest(nil, req)
	if err != nil {
		return err
	}

	requestID, err := cl.RequestCertificate(req, "")
	if err != nil {
		return err
	}

	pickupReq := &certificate.Request{
		PickupID: requestID,
		//TODO: make timeout configurable
		Timeout: 180 * time.Second,
	}
	d.Set("certificate_dn", requestID)

	//Workaround for VEN-46960
	time.Sleep(2 * time.Second)

	pcc, err := cl.RetrieveCertificate(pickupReq)
	if err != nil {
		return err
	}

	if pass, ok := d.GetOk("key_password"); ok {
		pcc.AddPrivateKey(req.PrivateKey, []byte(pass.(string)))
	} else {
		pcc.AddPrivateKey(req.PrivateKey, []byte(""))
	}

	if err = d.Set("certificate", pcc.Certificate); err != nil {
		return fmt.Errorf("Error setting certificate: %s", err)
	}
	log.Println("Certificate set to ", pcc.Certificate)

	if err = d.Set("chain", strings.Join((pcc.Chain), "")); err != nil {
		return fmt.Errorf("error setting chain: %s", err)
	}
	log.Println("Certificate chain set to", pcc.Chain)

	d.SetId(req.PickupID)
	log.Println("Setting up private key")
	d.Set("private_key_pem", pcc.PrivateKey)
	return nil
}
