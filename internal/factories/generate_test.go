package factories

import (
	"math/rand/v2"
	"testing"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// nilPool is a no-op FKPool used to prevent nil-pointer panics in fkRef.Generate.
type nilPool struct{}

func (nilPool) Pick(_, _, _ string, _ *rand.Rand) (any, bool) { return nil, false }

// smokeCtx builds a GenContext suitable for smoke-testing Generate.
// elem is passed as params["element"] for arrayMech.
func smokeCtx(dataType, udtName, elem string) seedapi.GenContext {
	params := map[string]any{}
	if elem != "" {
		params["element"] = elem
	}
	return seedapi.GenContext{
		Column: seedapi.Column{DataType: dataType, UDTName: udtName},
		Row:    0,
		Rng:    rand.New(rand.NewPCG(1, 2)),
		Params: seedapi.Params(params),
		FKPool: nilPool{},
	}
}

// TestGenerateSmoke calls Generate on every builtin factory and verifies the
// result is not nil. It intentionally uses a fixed seed so output is
// deterministic. Factories that may legitimately return nil (e.g. fkRef when
// the pool is empty) are excluded and tested separately.
func TestGenerateSmoke(t *testing.T) {
	cases := []struct {
		name     string
		factory  seedapi.Factory
		dataType string
		udtName  string
		elem     string // for array
	}{
		{name: "bool", factory: boolMech{}, dataType: "boolean"},
		{name: "city", factory: cityMech{}, dataType: "text"},
		{name: "company", factory: companyMech{}, dataType: "text"},
		{name: "country", factory: countryMech{}, dataType: "text"},
		{name: "first_name", factory: firstName{}, dataType: "text"},
		{name: "last_name", factory: lastName{}, dataType: "text"},
		{name: "full_name", factory: fullName{}, dataType: "text"},
		{name: "email", factory: emailMech{}, dataType: "text"},
		{name: "phone", factory: phoneMech{}, dataType: "text"},
		{name: "url", factory: urlMech{}, dataType: "text"},
		{name: "image_url", factory: imageURLMech{}, dataType: "text"},
		{name: "hostname", factory: hostnameMech{}, dataType: "text"},
		{name: "slug", factory: slugMech{}, dataType: "text"},
		{name: "color", factory: colorMech{}, dataType: "text"},
		{name: "title", factory: titleMech{}, dataType: "text"},
		{name: "gender", factory: genderMech{}, dataType: "text"},
		{name: "token", factory: tokenMech{}, dataType: "text"},
		{name: "ip_address", factory: ipAddressMech{}, dataType: "text"},
		{name: "filename", factory: filenameMech{}, dataType: "text"},
		{name: "mime_type", factory: mimeTypeMech{}, dataType: "text"},
		{name: "username", factory: usernameMech{}, dataType: "text"},
		{name: "currency", factory: currencyMech{}, dataType: "text"},
		{name: "language_code", factory: languageCodeMech{}, dataType: "text"},
		{name: "uuid", factory: uuidMech{}, dataType: "uuid"},
		{name: "position", factory: positionMech{}, dataType: "integer"},
		{name: "version_int", factory: versionIntMech{}, dataType: "integer"},
		{name: "level", factory: levelMech{}, dataType: "integer"},
		{name: "year", factory: yearMech{}, dataType: "integer"},
		{name: "priority", factory: priorityMech{}, dataType: "integer"},
		{name: "percentage", factory: percentageMech{}, dataType: "integer"},
		{name: "counter", factory: counterMech{}, dataType: "integer"},
		{name: "port", factory: portMech{}, dataType: "integer"},
		{name: "status_code", factory: statusCodeMech{}, dataType: "integer"},
		{name: "integer", factory: intMech{}, dataType: "integer"},
		{name: "decimal", factory: decimalMech{}, dataType: "numeric"},
		{name: "amount", factory: amountMech{}, dataType: "numeric"},
		{name: "file_size", factory: fileSizeMech{}, dataType: "bigint"},
		{name: "duration", factory: durationMech{}, dataType: "integer"},
		{name: "checksum", factory: checksumMech{}, dataType: "text"},
		{name: "version_str", factory: versionStrMech{}, dataType: "text"},
		{name: "latitude", factory: latitudeMech{}, dataType: "numeric"},
		{name: "longitude", factory: longitudeMech{}, dataType: "numeric"},
		{name: "bytea", factory: byteaMech{}, dataType: "bytea"},
		{name: "timestamp", factory: timestampMech{}, dataType: "timestamp"},
		{name: "date", factory: dateMech{}, dataType: "date"},
		{name: "timestamp_str", factory: timestampStrMech{}, dataType: "text"},
		{name: "time_of_day", factory: timeOfDayMech{}, dataType: "time"},
		{name: "pg_interval", factory: pgIntervalMech{}, dataType: "interval"},
		{name: "tstzrange", factory: tstzrangeMech{}, dataType: "tstzrange"},
		{name: "point", factory: pointMech{}, dataType: "point"},
		{name: "hstore", factory: hstoreMech{}, dataType: "hstore"},
		{name: "json_any", factory: jsonAny{}, dataType: "jsonb"},
		{name: "string", factory: textMech{}, dataType: "text"},
		{name: "patronymic", factory: patronymicMech{}, dataType: "text"},
		{name: "pk_serial", factory: pkSerial{}, dataType: "integer"},
		{name: "localized_json", factory: localizedJSON{}, dataType: "jsonb"},
		{name: "array_text", factory: arrayMech{}, dataType: "ARRAY", udtName: "_text", elem: "_text"},
		{name: "array_int4", factory: arrayMech{}, dataType: "ARRAY", udtName: "_int4", elem: "_int4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := smokeCtx(tc.dataType, tc.udtName, tc.elem)
			got := tc.factory.Generate(ctx)
			if got == nil {
				t.Fatalf("%s.Generate returned nil", tc.name)
			}
		})
	}
}

// TestGenerateFKRefEmptyPool confirms fkRef returns nil (not panic) when the
// pool has nothing to pick.
func TestGenerateFKRefEmptyPool(t *testing.T) {
	ctx := smokeCtx("integer", "", "")
	ctx.Column.FKTarget = "public.parent.id"
	ctx.Params = seedapi.Params(map[string]any{"target": "public.parent.id"})
	got := fkRef{}.Generate(ctx)
	if got != nil {
		t.Fatalf("fkRef with empty pool must return nil, got %v", got)
	}
}

// TestGenerateEnumValueStr verifies the factory picks from the provided list.
func TestGenerateEnumValueStr(t *testing.T) {
	ctx := smokeCtx("text", "", "")
	ctx.Params = seedapi.Params(map[string]any{"values": []any{"a", "b", "c"}})
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		v, ok := enumValueStr{}.Generate(ctx).(string)
		if !ok {
			t.Fatalf("EnumValueStr.Generate returned non-string")
		}
		seen[v] = true
		// advance row to vary rng
		ctx.Row = i
	}
	for _, want := range []string{"a", "b", "c"} {
		if !seen[want] {
			t.Errorf("EnumValueStr never produced %q", want)
		}
	}
}

// TestGenerateUUIDFormat verifies the uuid factory produces RFC 4122-ish output.
func TestGenerateUUIDFormat(t *testing.T) {
	ctx := smokeCtx("uuid", "", "")
	v, ok := uuidMech{}.Generate(ctx).(string)
	if !ok {
		t.Fatal("uuid Generate returned non-string")
	}
	if len(v) != 36 {
		t.Fatalf("uuid length = %d, want 36: %q", len(v), v)
	}
	if v[8] != '-' || v[13] != '-' || v[18] != '-' || v[23] != '-' {
		t.Fatalf("uuid not in 8-4-4-4-12 format: %q", v)
	}
}
