package main

// utils module
//
// Copyright (c) 2023 - Valentin Kuznetsov <vkuznet@gmail.com>
//
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	authz "github.com/CHESSComputing/golib/authz"
	srvConfig "github.com/CHESSComputing/golib/config"
	ldap "github.com/CHESSComputing/golib/ldap"
	services "github.com/CHESSComputing/golib/services"
)

// helper function to get new token for given user and scope
func newToken(user, scope string) (string, error) {
	customClaims := authz.CustomClaims{User: user, Scope: scope, Kind: "client_credentials", Application: "FOXDEN"}
	duration := srvConfig.Config.Authz.TokenExpires
	if duration == 0 {
		duration = 7200
	}
	return authz.JWTAccessToken(srvConfig.Config.Authz.ClientID, duration, customClaims)
}

// helper function to get provenance data
func getData(api, did string) ([]map[string]any, error) {
	var records []map[string]any
	// search request to DataDiscovery service
	rurl := fmt.Sprintf("%s/%s?did=%s", srvConfig.Config.Services.DataBookkeepingURL, api, did)
	resp, err := _httpReadRequest.Get(rurl)
	if err != nil {
		return records, err
	}
	// parse data records from meta-data service
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return records, err
	}
	if Verbose > 0 {
		log.Println("provenance data\n", string(data))
	}
	err = json.Unmarshal(data, &records)
	return records, err
}

// columnNames converts JSON attributes to column names
func columnNames(attrs []string) []string {
	var out []string
	for _, attr := range attrs {
		var camel string
		words := strings.Split(attr, "_")
		for _, word := range words {
			camel += strings.Title(word)
		}
		out = append(out, camel)
	}
	return out
}

// helper function to get CHESS user attributes
func chessAttributes(user string) (ldap.Entry, error) {
	var attrs ldap.Entry

	// obtain valid token
	_httpReadRequest.GetToken()

	// make call to Authz server to obtain user attributes
	rurl := fmt.Sprintf("%s/attrs?user=%s", srvConfig.Config.Services.AuthzURL, user)
	resp, err := _httpReadRequest.Get(rurl)
	if err != nil {
		return attrs, err
	}
	// parse data records from meta-data service
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return attrs, err
	}
	err = json.Unmarshal(data, &attrs)
	return attrs, err
}

// helper function to obtain chunk of records for given service request
func numberOfRecords(rec services.ServiceRequest) (int, error) {
	var total int

	// obtain valid token
	_httpReadRequest.GetToken()

	// based on user query process request from all FOXDEN services
	data, err := json.Marshal(rec)
	if err != nil {
		log.Println("ERROR: marshall error", err)
		return total, err
	}
	rurl := fmt.Sprintf("%s/nrecords", srvConfig.Config.Services.DiscoveryURL)
	resp, err := _httpReadRequest.Post(rurl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Println("ERROR: HTTP POST error", err)
		return total, err
	}
	// parse data records from discovery service
	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Println("ERROR: IO error", err)
		return total, err
	}
	var response services.ServiceResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		log.Println("ERROR: unable to unmarshal response", err)
		return total, err
	}
	if response.HttpCode != http.StatusOK {
		log.Println("ERROR", response.Error)
		return 0, err
	}
	return response.Results.NRecords, nil
}

// helper function to obtain chunk of records for given service request
func chunkOfRecords(rec services.ServiceRequest) (services.ServiceResponse, error) {
	var response services.ServiceResponse

	// obtain valid token
	_httpReadRequest.GetToken()

	// based on user query process request from all FOXDEN services
	data, err := json.Marshal(rec)
	if err != nil {
		log.Println("ERROR: marshall error", err)
		return response, err
	}
	rurl := fmt.Sprintf("%s/search", srvConfig.Config.Services.DiscoveryURL)
	resp, err := _httpReadRequest.Post(rurl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Println("ERROR: HTTP POST error", err)
		return response, err
	}
	// parse data records from discovery service
	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Println("ERROR: IO error", err)
		return response, err
	}
	err = json.Unmarshal(data, &response)
	return response, err
}
