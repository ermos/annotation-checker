package httpchecker

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ermos/annotation/parser"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type Result struct {
	// Res
	Status 	int
	Params	map[string]interface{}
	Queries map[string]interface{}
	Payload map[string]interface{}
	// Core
	r *http.Request
	a parser.API
	ps map[string]string
}

// Check and compare request values with API annotation
func Check(r *http.Request, a parser.API, params map[string]string) (res Result, err error) {
	res = Result {
		r: r,
		a: a,
		ps: params,
	}

	if err = res.checkParams(); err != nil {
		res.Status = http.StatusBadRequest
		return
	}

	if err = res.checkQueries(); err != nil {
		res.Status = http.StatusBadRequest
		return
	}

	if r.Method == "POST" || r.Method == "PUT" {
		ct := strings.Split(r.Header.Get("Content-Type"), ";")

		switch strings.ToLower(ct[0]) {
		case "application/json":
			if err = res.checkPayloadJSON(); err != nil {
				res.Status = http.StatusBadRequest
				return
			}
		default:
			res.Status = http.StatusBadRequest
			err = fmt.Errorf("%s is not supported by this API", ct[0])
		}
	}

	return
}

// checkParams allows to check request params
func (r Result) checkParams() (err error) {
	var value interface{}

	r.Params = make(map[string]interface{})

	for _, param := range r.a.Validate.Params {
		value = nil

		for k, v := range r.ps {
			if k == param.Key {
				value, err = convert(param.Type, v)
				if err != nil {
					return
				}
			}
		}

		r.Params[param.Key] = value
	}

	return
}

// checkQueries allows to check request queries
func (r Result) checkQueries() (err error) {
	var value interface{}

	list := make(map[string]string)
	r.Queries = make(map[string]interface{})

	split := strings.Split(r.r.URL.String(), "?")

	if len(split) < 2 {
		return
	}

	queries := strings.Split(split[1], "&")
	for _, q := range queries {
		split = strings.Split(q, "=")
		if len(split) == 1 {
			list[strings.ToLower(split[0])] = split[0]
		} else {
			list[strings.ToLower(split[0])] = split[1]
		}
	}

	for _, query := range r.a.Validate.Queries {
		value = nil

		if list[query.Key] == "" && !query.Nullable {
			return fmt.Errorf("%s's queries can't be empty", query.Key)
		}

		if list[query.Key] != "" {
			value, err = convert(query.Type, list[query.Key])
			if err != nil {
				return
			}
		}

		r.Queries[query.Key] = value
	}

	return
}

// checkPayloadJSON allows to check request payload (from JSON)
func (r Result) checkPayloadJSON() (err error) {
	var value interface{}
	var data map[string]interface{}

	r.Payload = make(map[string]interface{})

	if len(r.a.Validate.Payload) <= 0 {
		return
	}

	err = parseBody(r.r, &data)
	if err != nil {
		return err
	}

	for _, body := range r.a.Validate.Payload {
		if !body.Nullable && (data[body.Key] == "" || data[body.Key] == nil) {
			return fmt.Errorf("%s's key is required in payload", body.Key)
		}

		if data[body.Key] == "" || data[body.Key] == nil {
			r.Payload[body.Key] = nil
			continue
		}

		value, err = convert(body.Type, data[body.Key])
		if err != nil {
			return
		}

		r.Payload[body.Key] = value
	}

	return
}

// convert allows to convert value into their wanted value
func convert(trueType string, value interface{}) (interface{}, error) {
	var valueString string

	switch value.(type) {
	case int:
		valueString = fmt.Sprintf("%d", value.(int))
	case bool:
		valueString = fmt.Sprintf("%t", value.(bool))
	case float64:
		if trueType != "int" {
			valueString = fmt.Sprintf("%2.f", value.(float64))
		}else{
			valueString = fmt.Sprintf("%0.f", value.(float64))
		}
	case string:
		valueString = value.(string)
	default:
		if trueType == "map" {
			marshal, err := json.Marshal(value)
			if err != nil {
				return nil, errors.New("can't parse map type")
			}

			valueString = string(marshal)
		}else{
			return nil, errors.New("type not found")
		}
	}

	switch strings.ToLower(trueType) {
	case "int":
		rInt, err := strconv.Atoi(valueString)
		if err != nil {
			return rInt, fmt.Errorf("cannot convert %s to %s", valueString, "int")
		}
		return rInt, nil
	case "float64":
		rFloat64, err := strconv.ParseFloat(valueString, 64)
		if err != nil {
			return rFloat64, fmt.Errorf("cannot convert %s to %s", valueString, "float64")
		}
		return rFloat64, nil
	case "bool":
		rBool, err := strconv.ParseBool(valueString)
		if err != nil {
			return rBool, fmt.Errorf("cannot convert %s to %s", valueString, "bool")
		}
		return rBool, nil
	case "string", "map", "empty":
		return valueString, nil
	default:
		return value, fmt.Errorf("%s's type is not supported", trueType)
	}
}

func parseBody(r *http.Request, v interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, v)
}