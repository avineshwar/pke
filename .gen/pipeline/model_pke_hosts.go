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

type PkeHosts struct {
	Name string `json:"name"`
	PrivateIP string `json:"privateIP"`
	Roles []string `json:"roles"`
}
