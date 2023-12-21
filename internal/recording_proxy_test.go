package internal

import (
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

func TestBson(t *testing.T) {

	var b bson.D

	jsonStr := `{"foo": "bar", "hello": "world", "pi": 3.14159}`

	err := bson.UnmarshalExtJSON([]byte(jsonStr), true, &b)

	if err != nil {
		t.Error(err)
	}

	t.Log(b)

}
