package common

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/getlantern/enproxy"
	"github.com/getlantern/keyman"
	"github.com/getlantern/tls"
)

const (
	MASQUERADE_AS = "cdnjs.com"
	ROOT_CA       = "-----BEGIN CERTIFICATE-----\nMIIDdTCCAl2gAwIBAgILBAAAAAABFUtaw5QwDQYJKoZIhvcNAQEFBQAwVzELMAkG\nA1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsTB1Jv\nb3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw05ODA5MDExMjAw\nMDBaFw0yODAxMjgxMjAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9i\nYWxTaWduIG52LXNhMRAwDgYDVQQLEwdSb290IENBMRswGQYDVQQDExJHbG9iYWxT\naWduIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDaDuaZ\njc6j40+Kfvvxi4Mla+pIH/EqsLmVEQS98GPR4mdmzxzdzxtIK+6NiY6arymAZavp\nxy0Sy6scTHAHoT0KMM0VjU/43dSMUBUc71DuxC73/OlS8pF94G3VNTCOXkNz8kHp\n1Wrjsok6Vjk4bwY8iGlbKk3Fp1S4bInMm/k8yuX9ifUSPJJ4ltbcdG6TRGHRjcdG\nsnUOhugZitVtbNV4FpWi6cgKOOvyJBNPc1STE4U6G7weNLWLBYy5d4ux2x8gkasJ\nU26Qzns3dLlwR5EiUWMWea6xrkEmCMgZK9FGqkjWZCrXgzT/LCrBbBlDSgeF59N8\n9iFo7+ryUp9/k5DPAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8E\nBTADAQH/MB0GA1UdDgQWBBRge2YaRQ2XyolQL30EzTSo//z9SzANBgkqhkiG9w0B\nAQUFAAOCAQEA1nPnfE920I2/7LqivjTFKDK1fPxsnCwrvQmeU79rXqoRSLblCKOz\nyj1hTdNGCbM+w6DjY1Ub8rrvrTnhQ7k4o+YviiY776BQVvnGCv04zcQLcFGUl5gE\n38NflNUVyRRBnMRddWQVDf9VMOyGj/8N7yy5Y0b2qvzfvGn9LhJIZJrglfCm7ymP\nAbEVtQwdpf5pLGkkeB6zpxxxYu7KyJesF12KwvhHhm4qxFYxldBniYUr+WymXUad\nDKqC5JlR3XC321Y9YeRq4VzW9v493kHMB65jUr9TU/Qr6cf9tveCX4XSQRjbgbME\nHMUfpIBvFSDJ3gyICh3WZlXi/EjJKSZp4A==\n-----END CERTIFICATE-----\n"
)

type FlashlightClient struct {
	UpstreamHost string
}

func (client *FlashlightClient) NewClient() *http.Client {

	enproxyConfig := &enproxy.Config{
		DialProxy: func(addr string) (net.Conn, error) {
			return tls.DialWithDialer(
				&net.Dialer{
					Timeout:   8 * time.Second,
					KeepAlive: 70 * time.Second,
				},
				"tcp", client.addressForServer(), client.clientTLSConfig())
			//return net.Dial("tcp", addressForServer())
		},
		NewRequest: func(host string, method string, body io.Reader) (req *http.Request, err error) {
			if host == "" {
				host = client.UpstreamHost
			}
			log.Println("Making new request!!!")
			return http.NewRequest(method, "http://"+host+"/", body)
		},
	}

	localClient := &http.Client{
		Transport: &http.Transport{
			// We disable keepalives because some servers pretend to support
			// keep-alives but close their connections immediately, which
			// causes an error inside ReverseProxy.  This is not an issue
			// for HTTPS because  the browser is responsible for handling
			// the problem, which browsers like Chrome and Firefox already
			// know to do.
			// See https://code.google.com/p/go/issues/detail?id=4677
			DisableKeepAlives: true,
			Dial: func(network, addr string) (net.Conn, error) {
				conn := &enproxy.Conn{
					Addr:   addr,
					Config: enproxyConfig,
				}
				err := conn.Connect()
				if err != nil {
					return nil, err
				}
				return conn, nil
			},
		},
	}
	return localClient
}

// Get the address to dial for reaching the server
func (client *FlashlightClient) addressForServer() string {
	return fmt.Sprintf("%s:%d", MASQUERADE_AS, 443)
}

// Build a tls.Config for the client to use in dialing server
func (client *FlashlightClient) clientTLSConfig() *tls.Config {
	tlsConfig := &tls.Config{
		ClientSessionCache:                  tls.NewLRUClientSessionCache(1000),
		SuppressServerNameInClientHandshake: true,
	}
	// Note - we need to suppress the sending of the ServerName in the client
	// handshake to make host-spoofing work with Fastly.  If the client Hello
	// includes a server name, Fastly checks to make sure that this matches the
	// Host header in the HTTP request and if they don't match, it returns a
	// 400 Bad Request error.
	if ROOT_CA != "" {
		caCert, err := keyman.LoadCertificateFromPEMBytes([]byte(ROOT_CA))
		if err != nil {
			log.Fatalf("Unable to load root ca cert: %s", err)
		}
		tlsConfig.RootCAs = caCert.PoolContainingCert()
	}
	return tlsConfig
}
