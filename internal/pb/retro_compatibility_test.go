package pb

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// 旧フィールド番号で保存したデータを読めることと、再保存しても番号が変わらないことを確認する。
func TestRetroMetadataPreservesLegacyWireFormat(t *testing.T) {
	var legacy []byte
	for _, field := range []struct {
		number protowire.Number
		value  string
	}{{5, "2026-11-02"}, {6, "2026-11-08"}, {12, "2026-11"}} {
		legacy = protowire.AppendTag(legacy, field.number, protowire.BytesType)
		legacy = protowire.AppendString(legacy, field.value)
	}
	legacy = protowire.AppendTag(legacy, 13, protowire.VarintType)
	legacy = protowire.AppendVarint(legacy, 1)
	var record ChunkRecord
	if err := proto.Unmarshal(legacy, &record); err != nil {
		t.Fatal(err)
	}
	if record.GetRetroStartDate() != "2026-11-02" || record.GetRetroEndDate() != "2026-11-08" || record.GetRetroMonth() != "2026-11" || record.GetRetroSequence() != 1 {
		t.Fatalf("legacy metadata lost: %v", &record)
	}
	encoded, err := proto.Marshal(&record)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, legacy) {
		t.Fatal("field rename changed the wire format")
	}
}

// ProtoJSONの出力名を維持し、既存のcamelCase入力も新しいGo型で読み取れることを確認する。
func TestRetroMetadataPreservesLegacyProtoJSON(t *testing.T) {
	legacy := []byte(`{"reflectionStartDate":"2026-11-02","reflectionEndDate":"2026-11-08","reflectionMonth":"2026-11","reflectionSequence":1}`)
	var record ChunkRecord
	if err := protojson.Unmarshal(legacy, &record); err != nil {
		t.Fatal(err)
	}
	if record.GetRetroMonth() != "2026-11" || record.GetRetroSequence() != 1 {
		t.Fatal("legacy JSON metadata lost")
	}
	encoded, err := protojson.Marshal(&record)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip ChunkRecord
	if err := protojson.Unmarshal(encoded, &roundTrip); err != nil || !proto.Equal(&record, &roundTrip) {
		t.Fatal("legacy JSON round trip changed metadata")
	}
	for _, name := range [][]byte{[]byte("reflectionStartDate"), []byte("reflectionEndDate"), []byte("reflectionMonth"), []byte("reflectionSequence")} {
		if !bytes.Contains(encoded, name) {
			t.Fatalf("legacy JSON name missing: %s", name)
		}
	}
}
