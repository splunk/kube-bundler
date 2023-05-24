/*
   Copyright 2023 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package managers

import (
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	v1alpha1 "github.com/splunk/kube-bundler/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretName       = "global-secret"
	defaultNamespace = "default"
)

type ParameterDesc struct {
	Value       string
	Default     string
	Description string
}

type ParameterManager struct {
	kbClient    KBClient
	installName string
	definitions []v1alpha1.ParameterDefinitionSpec
	parameters  []v1alpha1.ParameterSpec
}

func NewParameterManager(kbClient KBClient, installName string, definitions []v1alpha1.ParameterDefinitionSpec, parameters []v1alpha1.ParameterSpec) *ParameterManager {
	return &ParameterManager{
		kbClient:    kbClient,
		installName: installName,
		definitions: definitions,
		parameters:  parameters,
	}
}

// GetMergedMap returns a parameter map with all overridden parameters merged in
func (pm *ParameterManager) GetMergedMap() (map[string]string, error) {
	m := make(map[string]string, len(pm.definitions))
	for _, parameter := range pm.definitions {
		if parameter.GenerateSecret.Format != "" {
			parameterSecretValue, err := pm.getSecretValue(parameter.Name, parameter.GenerateSecret)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get secret value")
			}
			m[parameter.Name] = parameterSecretValue
		} else {
			m[parameter.Name] = parameter.Default
		}
	}

	// Apply overrides
	for _, parameter := range pm.parameters {
		if parameter.GenerateSecret.Format != "" {
			parameterSecretValue, err := pm.getSecretValue(parameter.Name, parameter.GenerateSecret)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get secret value")
			}
			m[parameter.Name] = parameterSecretValue
		} else {
			m[parameter.Name] = parameter.Value
		}
	}

	return m, nil
}

// GetParameterDesc returns a map of Parameter structs where the map key is the name
func (pm *ParameterManager) GetParameterDesc() map[string]ParameterDesc {
	m := make(map[string]ParameterDesc, len(pm.definitions))
	for _, parameter := range pm.definitions {
		m[parameter.Name] = ParameterDesc{Value: parameter.Default, Default: parameter.Default, Description: parameter.Description}
	}

	// Apply overrides
	for _, parameter := range pm.parameters {
		desc := m[parameter.Name]
		desc.Value = parameter.Value
		m[parameter.Name] = desc
	}

	return m
}

func (pm *ParameterManager) MergeAdditionalParameters(additionalParameters []v1alpha1.ParameterSpec) []v1alpha1.ParameterSpec {

	found := false
	sourceParameters := pm.parameters
	paramsArray := []v1alpha1.ParameterSpec{}

	for _, additionalParameter := range additionalParameters {
		for _, params := range sourceParameters {
			if params.Name == additionalParameter.Name {
				params.Value = additionalParameter.Value
				found = true
				break
			}
		}
		if !found {
			paramSpec := v1alpha1.ParameterSpec{
				Name:  additionalParameter.Name,
				Value: additionalParameter.Value,
			}
			paramsArray = append(paramsArray, paramSpec)
		}
	}
	sourceParameters = append(sourceParameters, paramsArray...)

	return sourceParameters
}

func (pm *ParameterManager) Validate() error {
	m := make(map[string]v1alpha1.ParameterSpec, len(pm.parameters))
	for _, parameter := range pm.parameters {
		m[parameter.Name] = v1alpha1.ParameterSpec{Name: parameter.Name, Value: parameter.Value}
	}

	for _, definition := range pm.definitions {
		parameterSpec := m[definition.Name]
		value := parameterSpec.Value
		if definition.Required && strings.TrimSpace(value) == "" {
			return fmt.Errorf("required parameter '%s' not set", definition.Name)
		}
	}

	return nil
}

func (pm *ParameterManager) getSecretValue(parameterName string, generateSecret v1alpha1.GenerateSecret) (string, error) {
	secret, err := getGlobalSecret(pm.kbClient)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get global secret")
	}

	// Check if install-name.parameter-name is in the global secret
	secretKey := pm.installName + "." + parameterName
	secretValue, found := secret.Data[secretKey]
	if found {
		return string(secretValue[:]), nil
	} else {
		// If not found, generate new secret value
		parameterSecretValue, err := generateSecretValue(generateSecret.Format, generateSecret.Bytes, generateSecret.Bits)
		if err != nil {
			return "", errors.Wrap(err, "Failed to generate secret value")
		}
		// Add the new secret value to the global secret
		secret.Data[secretKey] = []byte(parameterSecretValue)
		clientset := pm.kbClient.Interface
		_, err = clientset.CoreV1().Secrets(defaultNamespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return "", errors.Wrap(err, "Failed to update global secret")
		}

		return parameterSecretValue, nil
	}
}

func getGlobalSecret(kbClient KBClient) (*corev1.Secret, error) {
	clientset := kbClient.Interface
	secretClient := clientset.CoreV1().Secrets(defaultNamespace)

	secret, err := secretClient.Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		// Global secret doesn't exist, create it
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{},
		}
		_, err = secretClient.Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create global secret")
		}
	}
	return secret, nil
}

func generateSecretValue(format string, bytes, bits int) (string, error) {

	var secretValue string

	if format == "hex" {
		randomBytes := make([]byte, bytes)
		_, err := crand.Read(randomBytes)
		if err != nil {
			return "", errors.Wrap(err, "Failed to generate random value")
		}
		secretValue = hex.EncodeToString(randomBytes)
	} else if format == "rsa" {
		rsaKey, err := generatePEMEncodedKey(bits)
		if err != nil {
			return "", errors.Wrap(err, "Failed to generate RSA key")
		}
		secretValue = string(rsaKey)
	} else {
		return "", errors.New("Unknown format: " + format)
	}

	return secretValue, nil
}

func generatePEMEncodedKey(bits int) ([]byte, error) {
	privKey, err := generateRandomPrivateKey(bits)
	if err != nil {
		return nil, err
	}
	return encodePrivateKeyToPEM(privKey)
}

func generateRandomPrivateKey(bits int) (*rsa.PrivateKey, error) {
	privKey, err := rsa.GenerateKey(crand.Reader, bits)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate private key")
	}
	err = privKey.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to validate private key")
	}
	return privKey, nil
}

func encodePrivateKeyToPEM(privKey *rsa.PrivateKey) ([]byte, error) {
	privBlock := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}
	pemEncodedKey := pem.EncodeToMemory(&privBlock)
	if pemEncodedKey == nil {
		return nil, errors.New("Unable to PEM-encode private key")
	}
	return pemEncodedKey, nil
}
