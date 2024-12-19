package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	srvConfig "github.com/CHESSComputing/golib/config"
	doi "github.com/CHESSComputing/golib/doi"
	services "github.com/CHESSComputing/golib/services"
)

func getMetaData(user, did string) (map[string]any, error) {
	var rec map[string]any
	token, err := newToken(user, "read")
	if err != nil {
		return rec, err
	}
	_httpReadRequest.Token = token
	query := fmt.Sprintf("{\"did\": \"%s\"}", did)
	srec := services.ServiceRequest{
		Client:       "foxden-doi",
		ServiceQuery: services.ServiceQuery{Query: query, Idx: 0, Limit: -1},
	}

	data, err := json.Marshal(srec)
	rurl := fmt.Sprintf("%s/search", srvConfig.Config.Services.MetaDataURL)
	resp, err := _httpReadRequest.Post(rurl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return rec, err
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return rec, err
	}
	var records []map[string]any
	err = json.Unmarshal(data, &records)
	if err != nil {
		return rec, err
	}
	if len(records) != 1 {
		return rec, errors.New("wrong number of records")
	}
	rec = records[0]
	return rec, nil
}

// helper function to publish did with given provider
func publishDataset(user, provider, did, description string) (string, string, error) {
	zenodoDoi := doi.ZenodoProvider{}
	mcDoi := doi.MCProvider{}
	dataciteDoi := doi.DataciteProvider{}

	// get meta-data record associated with did
	record, err := getMetaData(user, did)
	if err != nil {
		return "", "", err
	}

	p := strings.ToLower(provider)
	var doi, doiLink string
	if p == "zenodo" {
		zenodoDoi.Init()
		doi, doiLink, err = zenodoDoi.Publish(did, description, record)
	} else if p == "materialcommons" {
		mcDoi.Init()
		doi, doiLink, err = mcDoi.Publish(did, description, record)
	} else if p == "datacite" {
		dataciteDoi.Init()
		doi, doiLink, err = dataciteDoi.Publish(did, description, record)
	} else {
		msg := fmt.Sprintf("Provider '%s' is not supported", provider)
		err = errors.New(msg)
	}
	return doi, doiLink, err
}

// helper function to update DOI information in FOXDEN MetaData service
func updateMetaDataDOI(did, doi, doiLink string) error {
	var err error

	// extract schema from did
	var schema string
	for _, part := range strings.Split(did, "/") {
		if strings.HasPrefix(part, "beamline=") {
			schema = strings.Replace(part, "beamline=", "", -1)
			break
		}
	}
	if strings.Contains(schema, ",") {
		msg := fmt.Sprintf("unsupported did=%s with multiple schemas %s for MetaData update", did, schema)
		return errors.New(msg)
	}

	// fetch records matching our did
	_httpReadRequest.GetToken()
	rurl := fmt.Sprintf("%s/record?did=%s", srvConfig.Config.Services.MetaDataURL, did)
	resp, err := _httpReadRequest.Get(rurl)
	defer resp.Body.Close()
	if err != nil {
		log.Println("ERROR: unable to GET to MetaData service, error", err)
		return err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("ERROR: unable to read response body, error", err)
		return err
	}
	var records []map[string]any
	err = json.Unmarshal(data, &records)
	if err != nil {
		log.Println("ERROR: unable to unmarshal service response, error", err)
		return err
	}

	// for all matching records perform update
	for _, rec := range records {
		// drop _id as it does not belong to the meta-data schema
		delete(rec, "_id")
		// and add doi attributes
		rec["doi"] = doi
		rec["doi_url"] = doiLink

		// create meta-data record for update
		mrec := services.MetaRecord{Schema: schema, Record: rec}

		// prepare http writer
		_httpWriteRequest.GetToken()

		// place request to MetaData service
		rurl := fmt.Sprintf("%s", srvConfig.Config.Services.MetaDataURL)
		data, err := json.Marshal(mrec)
		if err != nil {
			log.Println("ERROR: unable to marshal meta-data record, error", err)
			return err
		}
		resp, err := _httpWriteRequest.Put(rurl, "application/json", bytes.NewBuffer(data))
		defer resp.Body.Close()
		if err != nil {
			log.Println("ERROR: unable to POST to MetaData service, error", err)
			return err
		}
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Println("ERROR: unable to read response body, error", err)
			return err
		}

		var sresp services.ServiceResponse
		err = json.Unmarshal(data, &sresp)
		if err != nil {
			log.Println("ERROR: unable to unmarshal service response, error", err)
			return err
		}
		if sresp.SrvCode != 0 || sresp.HttpCode != http.StatusOK {
			return errors.New(sresp.String())
		}
	}
	return nil
}
