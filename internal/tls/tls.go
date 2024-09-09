package tls

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"software.sslmate.com/src/go-pkcs12"
)

const password = "hazelcast"

func HazelcastKeyAndTrustStore(ctx context.Context, c client.Client, secretName, namespace string) ([]byte, []byte, error) {
	s := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, s)
	if err != nil {
		return nil, nil, err
	}

	cert, caCerts, err := extractCerts(s.Data["tls.crt"])
	if err != nil {
		return nil, nil, err
	}

	key, err := extractKey(s.Data["tls.key"])
	if err != nil {
		return nil, nil, err
	}

	encoder := pkcs12.Modern2023
	keyStore, err := encoder.Encode(key, cert, caCerts, password)
	if err != nil {
		return nil, nil, err
	}

	cas, err := loadCAs(s)
	if err != nil {
		return nil, nil, err
	}

	var trustStore []byte
	if len(cas) == 0 {
		trustStore = keyStore
	} else {
		trustStore, err = encoder.EncodeTrustStore(cas, password)
		if err != nil {
			return nil, nil, err
		}
	}

	return keyStore, trustStore, nil
}

func loadCAs(secret *corev1.Secret) ([]*x509.Certificate, error) {
	certs, err := extractCACerts(secret.Data["ca.crt"])
	if err != nil {
		return nil, err
	}
	return certs, nil
}

func extractKey(data []byte) (any, error) {
	b, _ := pem.Decode(data)
	if b == nil {
		return nil, fmt.Errorf("expected at least one pem block")
	}
	if b.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("expected type %v, got %v", "PRIVATE KEY", b.Type)
	}
	pvtKey, err := x509.ParsePKCS8PrivateKey(b.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing private key")
	}
	return pvtKey, nil
}

func extractCerts(certData []byte) (*x509.Certificate, []*x509.Certificate, error) {
	var cert *x509.Certificate
	var cas []*x509.Certificate
	for {
		var block *pem.Block
		block, certData = pem.Decode(certData)
		if block == nil {
			break // No more blocks
		}
		// Parse the certificate
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot parse certificate: %s", block.Type)
		}

		if c.IsCA {
			cas = append(cas, c)
		} else {
			cert = c
		}
		// if there is a single certificate use it as the server certificate
		if len(cas) == 1 && cert == nil {
			cert = cas[0]
			cas = nil
		}
	}
	return cert, cas, nil
}

func extractCACerts(certData []byte) ([]*x509.Certificate, error) {
	var cas []*x509.Certificate
	for {
		var block *pem.Block
		block, certData = pem.Decode(certData)
		if block == nil {
			break
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("cannot parse certificate: %s", block.Type)
		}

		cas = append(cas, c)
	}
	return cas, nil
}
