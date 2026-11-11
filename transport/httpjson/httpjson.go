// Package httpjson writes contract messages as JSON responses, one or a list, the way protojson spells them.
package httpjson

import (
	"bytes"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func WriteOne(w http.ResponseWriter, message proto.Message) {
	data, _ := protojson.Marshal(message)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func WriteList[T proto.Message](w http.ResponseWriter, items []T) {
	var body bytes.Buffer
	body.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			body.WriteByte(',')
		}
		data, _ := protojson.Marshal(item)
		body.Write(data)
	}
	body.WriteByte(']')
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body.Bytes())
}
