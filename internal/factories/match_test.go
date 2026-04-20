package factories

import (
	"testing"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// TestBuiltinMatchTiers pins the contract "which MatchScore does each generic
// factory return for key column name variants". It catches regressions where
// WeakNameMatch and NameMatch are accidentally swapped for a factory, or where
// name predicates are weakened (timestamp patterns, integer enum/orphan-FK gates).
func TestBuiltinMatchTiers(t *testing.T) {
	fkTarget := "public.users.id"

	cases := []struct {
		name string
		fac  seedapi.Matcher
		col  seedapi.Column
		want seedapi.MatchScore
	}{
		// bool: always WeakNameMatch — type is unambiguous but a NameMatch plugin still wins.
		{"bool/boolean", boolMech{}, seedapi.Column{DataType: "boolean"}, seedapi.WeakNameMatch},
		{"bool/not-bool", boolMech{}, seedapi.Column{DataType: "integer"}, seedapi.NoMatch},

		// date: any date type → WeakNameMatch.
		{"date/date", dateMech{}, seedapi.Column{DataType: "date"}, seedapi.WeakNameMatch},
		{"date/not-date", dateMech{}, seedapi.Column{DataType: "integer"}, seedapi.NoMatch},

		// decimal: any numeric/float type → WeakNameMatch.
		{"decimal/numeric", decimalMech{}, seedapi.Column{DataType: "numeric"}, seedapi.WeakNameMatch},
		{"decimal/double", decimalMech{}, seedapi.Column{DataType: "double precision"}, seedapi.WeakNameMatch},
		{"decimal/not-numeric", decimalMech{}, seedapi.Column{DataType: "integer"}, seedapi.NoMatch},

		// hstore: UDT hstore → WeakNameMatch.
		{"hstore/hstore", hstoreMech{}, seedapi.Column{UDTName: "hstore"}, seedapi.WeakNameMatch},
		{"hstore/not-hstore", hstoreMech{}, seedapi.Column{DataType: "jsonb"}, seedapi.NoMatch},

		// timestamp: name-pattern match → WeakNameMatch; bare column → TypeMatch (unresolved).
		{"ts/created_at", timestampMech{}, seedapi.Column{Name: "created_at", DataType: "timestamp"}, seedapi.WeakNameMatch},
		{"ts/updated_at", timestampMech{}, seedapi.Column{Name: "updated_at", DataType: "timestamp"}, seedapi.WeakNameMatch},
		{"ts/applied_on", timestampMech{}, seedapi.Column{Name: "applied_on", DataType: "timestamp"}, seedapi.WeakNameMatch},
		{"ts/start_date", timestampMech{}, seedapi.Column{Name: "start_date", DataType: "timestamp"}, seedapi.WeakNameMatch},
		{"ts/action_time", timestampMech{}, seedapi.Column{Name: "action_time", DataType: "timestamp"}, seedapi.WeakNameMatch},
		{"ts/deadline", timestampMech{}, seedapi.Column{Name: "deadline", DataType: "timestamp"}, seedapi.WeakNameMatch},
		{"ts/applied", timestampMech{}, seedapi.Column{Name: "applied", DataType: "timestamp"}, seedapi.WeakNameMatch},
		// `time_zone` does NOT match the `_time$` suffix pattern, so it stays TypeMatch.
		{"ts/time_zone-no-match", timestampMech{}, seedapi.Column{Name: "time_zone", DataType: "timestamp"}, seedapi.TypeMatch},
		{"ts/bare-timestamp", timestampMech{}, seedapi.Column{Name: "ts", DataType: "timestamp"}, seedapi.TypeMatch},
		{"ts/not-timestamp", timestampMech{}, seedapi.Column{Name: "created_at", DataType: "integer"}, seedapi.NoMatch},

		// integer: ordinary columns → WeakNameMatch; orphan *_id and enum status/type → TypeMatch.
		{"int/counter", intMech{}, seedapi.Column{Name: "counter", DataType: "integer"}, seedapi.WeakNameMatch},
		{"int/items", intMech{}, seedapi.Column{Name: "items", DataType: "integer"}, seedapi.WeakNameMatch},
		// `_id` suffix with no FKTarget → TypeMatch (user should attach fkref).
		{"int/user_id-no-fk", intMech{}, seedapi.Column{Name: "user_id", DataType: "integer"}, seedapi.TypeMatch},
		{"int/user_id-with-fk", intMech{}, seedapi.Column{Name: "user_id", DataType: "integer", FKTarget: fkTarget}, seedapi.WeakNameMatch},
		// Plain `id` — HasSuffix("id","_id") is false so the `*_id` gate does not
		// fire. pkSerial will win via StrongMatch anyway, but here we verify intMech
		// alone returns WeakNameMatch.
		{"int/id", intMech{}, seedapi.Column{Name: "id", DataType: "integer"}, seedapi.WeakNameMatch},
		// Plural `_ids` does not match the `_id` suffix gate — stays WeakNameMatch.
		{"int/something_ids", intMech{}, seedapi.Column{Name: "something_ids", DataType: "integer"}, seedapi.WeakNameMatch},
		// Enum candidates: status/type as a whole word.
		{"int/status", intMech{}, seedapi.Column{Name: "status", DataType: "integer"}, seedapi.TypeMatch},
		{"int/order_status", intMech{}, seedapi.Column{Name: "order_status", DataType: "integer"}, seedapi.TypeMatch},
		{"int/status_code", intMech{}, seedapi.Column{Name: "status_code", DataType: "integer"}, seedapi.TypeMatch},
		{"int/type", intMech{}, seedapi.Column{Name: "type", DataType: "integer"}, seedapi.TypeMatch},
		{"int/type_code", intMech{}, seedapi.Column{Name: "type_code", DataType: "integer"}, seedapi.TypeMatch},
		{"int/role_type", intMech{}, seedapi.Column{Name: "role_type", DataType: "integer"}, seedapi.TypeMatch},
		// False-positive guards: `prototype_*` and `subtype_*` must NOT be treated as enum.
		{"int/prototype_number", intMech{}, seedapi.Column{Name: "prototype_number", DataType: "integer"}, seedapi.WeakNameMatch},
		{"int/subtype_sequence", intMech{}, seedapi.Column{Name: "subtype_sequence", DataType: "integer"}, seedapi.WeakNameMatch},
		{"int/archetype_depth", intMech{}, seedapi.Column{Name: "archetype_depth", DataType: "integer"}, seedapi.WeakNameMatch},

		// pk_serial: case-insensitive match for "id".
		{"pk/id-lower", pkSerial{}, seedapi.Column{Name: "id", DataType: "integer"}, seedapi.StrongMatch},
		{"pk/Id-mixed", pkSerial{}, seedapi.Column{Name: "Id", DataType: "integer"}, seedapi.StrongMatch},
		{"pk/ID-upper", pkSerial{}, seedapi.Column{Name: "ID", DataType: "integer"}, seedapi.StrongMatch},
		{"pk/user_id", pkSerial{}, seedapi.Column{Name: "user_id", DataType: "integer"}, seedapi.NoMatch},
		{"pk/id-not-int", pkSerial{}, seedapi.Column{Name: "id", DataType: "uuid"}, seedapi.NoMatch},

		// Named numeric factories must return NameMatch. If their score were lowered
		// to WeakNameMatch, registration order in factories.All() would become
		// load-bearing again — this test catches that regression directly.
		{"amount/payment_amount", amountMech{}, seedapi.Column{Name: "payment_amount", DataType: "numeric"}, seedapi.NameMatch},
		{"amount/discount_amount", amountMech{}, seedapi.Column{Name: "discount_amount", DataType: "integer"}, seedapi.NameMatch},
		{"amount/not-numeric", amountMech{}, seedapi.Column{Name: "amount", DataType: "text"}, seedapi.NoMatch},
		{"percentage/user_score", percentageMech{}, seedapi.Column{Name: "user_score", DataType: "integer"}, seedapi.NameMatch},
		{"percentage/progress", percentageMech{}, seedapi.Column{Name: "progress", DataType: "integer"}, seedapi.NameMatch},
		{"counter/user_count", counterMech{}, seedapi.Column{Name: "user_count", DataType: "integer"}, seedapi.NameMatch},
		{"year/birth_year", yearMech{}, seedapi.Column{Name: "birth_year", DataType: "integer"}, seedapi.NameMatch},
		{"latitude/latitude", latitudeMech{}, seedapi.Column{Name: "latitude", DataType: "numeric"}, seedapi.NameMatch},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fac.Match(seedapi.MatchContext{Column: tc.col})
			if got != tc.want {
				t.Fatalf("Match(%+v) = %d, want %d", tc.col, got, tc.want)
			}
		})
	}
}
