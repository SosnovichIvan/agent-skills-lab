// Package httpapi contains shared HTTP transport helpers.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const MaxJSONBodyBytes int64 = 1 << 20

// DecodeJSON decodes exactly one JSON value and rejects unknown object fields.
func DecodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body contains multiple JSON values")
		}
		return err
	}
	return nil
}

// LimitJSONBody limits request bodies before they reach handlers.
func LimitJSONBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBodyBytes)
		next.ServeHTTP(w, r)
	})
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func WriteData(w http.ResponseWriter, status int, data any) {
	body, err := MarshalData(data)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	WriteBytes(w, status, body)
}

func MarshalData(data any) ([]byte, error) { return json.Marshal(dataEnvelope{Data: data}) }

func WriteError(w http.ResponseWriter, status int, code, message string) {
	requestID := w.Header().Get("X-Request-ID")
	WriteJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message, RequestID: requestID}})
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		return
	}
	WriteBytes(w, status, body)
}

func WriteBytes(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
