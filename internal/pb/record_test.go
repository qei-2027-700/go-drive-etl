package pb

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// 名前をretroへ変えても、既存フィールドの番号・型・optionalを変えないことを確認する。
// 実際にBigQueryへ保存する列名と値は、ETLのテストで確認する。
func TestChunkRecordRetroFieldCompatibility(t *testing.T) {
	fields := (&ChunkRecord{}).ProtoReflect().Descriptor().Fields()
	for _, tc := range []struct {
		name   protoreflect.Name
		number protoreflect.FieldNumber
		kind   protoreflect.Kind
	}{
		{"retro_start_date", 5, protoreflect.StringKind},
		{"retro_end_date", 6, protoreflect.StringKind},
		{"retro_month", 12, protoreflect.StringKind},
		{"retro_sequence", 13, protoreflect.Int32Kind},
	} {
		t.Run(string(tc.name), func(t *testing.T) {
			field := fields.ByName(tc.name)
			if field == nil {
				t.Fatal("field is missing")
			}
			if field.Number() != tc.number || field.Kind() != tc.kind || !field.HasOptionalKeyword() || field.Cardinality() != protoreflect.Optional {
				t.Fatalf("got number=%d kind=%s cardinality=%s optional=%t; want number=%d kind=%s optional field", field.Number(), field.Kind(), field.Cardinality(), field.HasOptionalKeyword(), tc.number, tc.kind)
			}
		})
	}
}
