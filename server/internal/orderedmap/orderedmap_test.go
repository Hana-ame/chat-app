package orderedmap_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/orderedmap"
)

func TestNew(t *testing.T) {
	o := orderedmap.New()
	if o == nil {
		t.Fatal("New returned nil")
	}
	if keys := o.Keys(); len(keys) != 0 {
		t.Fatal("new map should have no keys")
	}
}

func TestNewOrderedMap(t *testing.T) {
	o := orderedmap.NewOrderedMap()
	if o == nil {
		t.Fatal("NewOrderedMap returned nil")
	}
}

func TestSetAndGet(t *testing.T) {
	o := orderedmap.New()
	o.Set("a", 1)
	o.Set("b", 2)

	val, ok := o.Get("a")
	if !ok || val != 1 {
		t.Fatal("Get a failed")
	}
	val, ok = o.Get("b")
	if !ok || val != 2 {
		t.Fatal("Get b failed")
	}
	_, ok = o.Get("c")
	if ok {
		t.Fatal("c should not exist")
	}
}

func TestSetOverride(t *testing.T) {
	o := orderedmap.New()
	o.Set("k", "v1")
	o.Set("k", "v2")
	if keys := o.Keys(); len(keys) != 1 {
		t.Fatalf("key count should be 1 after override, got %d", len(keys))
	}
	val, _ := o.Get("k")
	if val != "v2" {
		t.Fatalf("want v2, got %v", val)
	}
}

func TestDelete(t *testing.T) {
	o := orderedmap.New()
	o.Set("a", 1)
	o.Set("b", 2)
	o.Set("c", 3)
	o.Delete("b")
	keys := o.Keys()
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "c" {
		t.Fatalf("keys after delete: %v", keys)
	}
	if _, ok := o.Get("b"); ok {
		t.Fatal("b should be deleted")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	o := orderedmap.New()
	o.Delete("nonexistent")
}

func TestKeysOrder(t *testing.T) {
	o := orderedmap.New()
	o.Set("z", 1)
	o.Set("y", 2)
	o.Set("x", 3)
	keys := o.Keys()
	if len(keys) != 3 || keys[0] != "z" || keys[1] != "y" || keys[2] != "x" {
		t.Fatalf("keys should maintain insertion order: %v", keys)
	}
}

func TestValues(t *testing.T) {
	o := orderedmap.New()
	o.Set("a", 1)
	vals := o.Values()
	if v, ok := vals["a"]; !ok || v != 1 {
		t.Fatal("Values failed")
	}
}

func TestSortKeys(t *testing.T) {
	o := orderedmap.New()
	o.Set("c", 3)
	o.Set("a", 1)
	o.Set("b", 2)
	o.SortKeys(func(keys []string) {
		for i := 0; i < len(keys)-1; i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
	})
	keys := o.Keys()
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Fatalf("sort keys: got %v", keys)
	}
}

func TestSortByValue(t *testing.T) {
	o := orderedmap.New()
	o.Set("a", 3)
	o.Set("b", 1)
	o.Set("c", 2)
	o.Sort(func(a, b *orderedmap.Pair) bool {
		return a.Value().(int) < b.Value().(int)
	})
	keys := o.Keys()
	if keys[0] != "b" || keys[1] != "c" || keys[2] != "a" {
		t.Fatalf("sort by value: got %v", keys)
	}
}

func TestNewFromPairs(t *testing.T) {
	pairs := []*orderedmap.Pair{
		orderedmap.NewPair("a", 1),
		orderedmap.NewPair("b", 2),
	}
	o := orderedmap.NewFromPairs(pairs)
	keys := o.Keys()
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatal("NewFromPairs failed")
	}
	val, _ := o.Get("a")
	if val != 1 {
		t.Fatal("NewFromPairs value wrong")
	}
}

func TestNewFromMap(t *testing.T) {
	m := map[string]interface{}{"b": 2, "a": 1}
	o := orderedmap.NewFromMap(m)
	if keys := o.Keys(); len(keys) != 2 {
		t.Fatalf("NewFromMap: want 2 keys, got %d", len(keys))
	}
	val, _ := o.Get("a")
	if val != 1 {
		t.Fatal("NewFromMap value wrong")
	}
}

func TestGetOrDefault(t *testing.T) {
	o := orderedmap.New()
	o.Set("a", 1)
	if v := o.GetOrDefault("a", 0); v != 1 {
		t.Fatalf("want 1, got %v", v)
	}
	if v := o.GetOrDefault("b", 0); v != 0 {
		t.Fatalf("want default 0, got %v", v)
	}
	o.Set("c", "string")
	if v := o.GetOrDefault("c", 0); v != 0 {
		t.Fatalf("want default on type mismatch, got %v", v)
	}
}

func TestPair(t *testing.T) {
	p := orderedmap.NewPair("k", "v")
	if p.Key() != "k" || p.Value() != "v" {
		t.Fatal("Pair basic test failed")
	}
}

func TestSetEscapeHTML(t *testing.T) {
	o := orderedmap.New()
	o.SetEscapeHTML(false)
	o.Set("a", "<b>bold</b>")
	data, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "<b>bold</b>") {
		t.Fatalf("expected unescaped HTML, got %s", s)
	}
	if strings.Contains(s, "\\u003c") {
		t.Fatalf("should not contain escaped HTML: %s", s)
	}
}

func TestEscapeHTMLDefault(t *testing.T) {
	o := orderedmap.New()
	o.Set("a", "<b>bold</b>")
	data, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "\\u003cb\\u003e") {
		t.Fatalf("expected escaped HTML, got %s", s)
	}
}

func TestMarshalJSON(t *testing.T) {
	o := orderedmap.New()
	o.Set("b", 2)
	o.Set("a", 1)
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["a"] != float64(1) || result["b"] != float64(2) {
		t.Fatal("marshal content wrong")
	}
}

func TestMarshalJSONNested(t *testing.T) {
	inner := orderedmap.New()
	inner.Set("x", 10)
	o := orderedmap.New()
	o.Set("outer", inner)
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"outer":{"x":10}}` {
		t.Fatalf("got %s", string(data))
	}
}

func TestUnmarshalJSON(t *testing.T) {
	raw := `{"b":2,"a":1}`
	var o orderedmap.OrderedMap
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	keys := o.Keys()
	if len(keys) != 2 || keys[0] != "b" || keys[1] != "a" {
		t.Fatalf("unmarshal should preserve order: %v", keys)
	}
	val, _ := o.Get("a")
	if val != float64(1) {
		t.Fatalf("value wrong: %v", val)
	}
}

func TestUnmarshalJSONNested(t *testing.T) {
	raw := `{"outer":{"inner":"value"},"list":[1,2,3]}`
	var o orderedmap.OrderedMap
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	keys := o.Keys()
	if len(keys) != 2 || keys[0] != "outer" || keys[1] != "list" {
		t.Fatalf("keys order wrong: %v", keys)
	}
	if inner, ok := o.Get("outer"); ok {
		if _, isMap := inner.(orderedmap.OrderedMap); !isMap {
			t.Fatal("nested object should be OrderedMap")
		}
	}
}

func TestUnmarshalJSONDuplicateKey(t *testing.T) {
	raw := `{"a":1,"b":2,"a":3}`
	var o orderedmap.OrderedMap
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	keys := o.Keys()
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %d", len(keys))
	}
	if keys[0] != "b" || keys[1] != "a" {
		t.Fatalf("last duplicate key should be at end: %v", keys)
	}
	val, _ := o.Get("a")
	if val != float64(3) {
		t.Fatalf("want 3, got %v", val)
	}
}

func TestReader(t *testing.T) {
	o := orderedmap.New()
	o.Set("a", 1)
	r, err := o.Reader()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 100)
	n, _ := r.Read(buf)
	content := string(buf[:n])
	if len(content) == 0 {
		t.Fatal("reader returned empty")
	}
}

func TestUnmarshalJSON_InvalidJSON(t *testing.T) {
	var o orderedmap.OrderedMap
	err := o.UnmarshalJSON([]byte(`{invalid}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnmarshalJSON_Truncated(t *testing.T) {
	var o orderedmap.OrderedMap
	err := o.UnmarshalJSON([]byte(`{"a":`))
	if err == nil {
		t.Fatal("expected error for truncated JSON")
	}
}

func TestUnmarshalJSONNestedObjectInArray(t *testing.T) {
	raw := `{"arr":[{"x":1},{"y":2}]}`
	var o orderedmap.OrderedMap
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	val, ok := o.Get("arr")
	if !ok {
		t.Fatal("missing arr key")
	}
	if _, isSlice := val.([]interface{}); !isSlice {
		t.Fatalf("arr should be []interface{}, got %T", val)
	}
}

func TestUnmarshalJSONNestedArrayInArray(t *testing.T) {
	raw := `{"matrix":[[1,2],[3,4]]}`
	var o orderedmap.OrderedMap
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalJSONTwiceNested(t *testing.T) {
	raw1 := `{"a":{"x":1},"b":2}`
	var o orderedmap.OrderedMap
	if err := json.Unmarshal([]byte(raw1), &o); err != nil {
		t.Fatal(err)
	}
	// Second unmarshal hits the OrderedMap value branch in decodeOrderedMap
	raw2 := `{"a":{"y":2},"b":3}`
	if err := json.Unmarshal([]byte(raw2), &o); err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalJSONSliceWithObjectsTwice(t *testing.T) {
	raw1 := `{"items":[{"id":1}]}`
	var o orderedmap.OrderedMap
	json.Unmarshal([]byte(raw1), &o)
	raw2 := `{"items":[{"id":2}]}`
	if err := json.Unmarshal([]byte(raw2), &o); err != nil {
		t.Fatal(err)
	}
}

func TestMarshalJSON_Error(t *testing.T) {
	o := orderedmap.New()
	o.Set("ch", make(chan int))
	_, err := o.MarshalJSON()
	if err == nil {
		t.Fatal("expected error for non-serializable value")
	}
}

func TestReader_Error(t *testing.T) {
	o := orderedmap.New()
	o.Set("ch", make(chan int))
	_, err := o.Reader()
	if err == nil {
		t.Fatal("expected error for non-serializable value")
	}
}

func TestByPairSort(t *testing.T) {
	pairs := []*orderedmap.Pair{
		orderedmap.NewPair("a", 3),
		orderedmap.NewPair("b", 1),
		orderedmap.NewPair("c", 2),
	}
	byPair := orderedmap.ByPair{Pairs: pairs, LessFunc: func(a, b *orderedmap.Pair) bool {
		return a.Value().(int) < b.Value().(int)
	}}
	if byPair.Len() != 3 {
		t.Fatal("Len wrong")
	}
	byPair.Swap(0, 1)
	if byPair.Pairs[0].Key() != "b" || byPair.Pairs[1].Key() != "a" {
		t.Fatal("Swap wrong")
	}
	if !byPair.Less(0, 1) {
		t.Fatal("Less wrong")
	}
}
