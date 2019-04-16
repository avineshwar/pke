/*
 * Pipeline API
 *
 * Pipeline v0.3.0 swagger
 *
 * API version: master
 * Contact: info@banzaicloud.com
 */

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package pipeline

type CreateSecretRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Tags []string `json:"tags,omitempty"`
	Version int32 `json:"version,omitempty"`
	Values map[string]interface{} `json:"values"`
}
