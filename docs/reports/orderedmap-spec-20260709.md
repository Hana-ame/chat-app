# orderedmap 规范

> 原始来源：`server/internal/orderedmap/orderedmap.go`

---

## 一、原始代码

```go
package orderedmap

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"sort"
)

type Pair struct {
	key   string
	value interface{}
}

func NewPair(key string, value interface{}) *Pair {
	return &Pair{key, value}
}

func (kv *Pair) Key() string   { return kv.key }
func (kv *Pair) Value() interface{} { return kv.value }

type ByPair struct {
	Pairs    []*Pair
	LessFunc func(a *Pair, j *Pair) bool
}

func (a ByPair) Len() int           { return len(a.Pairs) }
func (a ByPair) Swap(i, j int)      { a.Pairs[i], a.Pairs[j] = a.Pairs[j], a.Pairs[i] }
func (a ByPair) Less(i, j int) bool { return a.LessFunc(a.Pairs[i], a.Pairs[j]) }

type OrderedMap struct {
	keys       []string
	values     map[string]interface{}
	escapeHTML bool
}

func New() *OrderedMap
func NewOrderedMap() *OrderedMap
func NewFromPairs(pairs []*Pair) *OrderedMap
func NewFromMap(values map[string]interface{}) *OrderedMap

func (o *OrderedMap) SetEscapeHTML(on bool)
func (o *OrderedMap) Get(key string) (interface{}, bool)
func (o *OrderedMap) GetOrDefault(key string, defaultValue interface{}) interface{}
func (o *OrderedMap) Set(key string, value interface{})
func (o *OrderedMap) Delete(key string)
func (o *OrderedMap) Keys() []string
func (o *OrderedMap) Values() map[string]interface{}
func (o *OrderedMap) SortKeys(sortFunc func(keys []string))
func (o *OrderedMap) Sort(lessFunc func(a *Pair, b *Pair) bool)
func (o *OrderedMap) UnmarshalJSON(b []byte) error
func (o OrderedMap) MarshalJSON() ([]byte, error)
func (o OrderedMap) Reader() (io.Reader, error)
```

---

## 二、类型总表

| 类型 | 说明 |
|------|------|
| `OrderedMap` | 保持插入顺序的 JSON map，keys 为 `[]string`，values 为 `map[string]interface{}` |
| `Pair` | key-value 对，用于排序和批量构造 |
| `ByPair` | `sort.Interface` 实现，用于自定义排序 |

---

## 三、方法总表

| 方法 | 说明 |
|------|------|
| `New()` | 创建空 OrderedMap |
| `NewFromPairs(pairs)` | 从 Pair 列表创建 |
| `NewFromMap(values)` | 从 map 创建（keys = 随机顺序） |
| `Set(key, value)` | 设置值（key 不存在时追加到 keys 末尾） |
| `Get(key)` | 读取值（返回 value + exists） |
| `GetOrDefault(key, default)` | 读取值，不存在或类型不匹配返回默认值 |
| `Delete(key)` | 删除 key |
| `Keys()` | 返回 keys 切片（有序） |
| `Values()` | 返回 values map |
| `SortKeys(sortFunc)` | 对 keys 排序 |
| `Sort(lessFunc)` | 按 Pair 排序 |
| `SetEscapeHTML(on)` | 控制 JSON 输出是否转义 HTML |
| `MarshalJSON()` | 按 keys 顺序输出 JSON |
| `UnmarshalJSON(b)` | 按输入顺序解析 JSON |
| `Reader()` | 返回有序 JSON 的 io.Reader |

---

## 四、MarshalJSON 行为

```
buf.WriteByte('{')
for i, k := range o.keys:
  if i > 0: buf.WriteByte(',')
  encoder.Encode(k)    // key
  buf.WriteByte(':')
  encoder.Encode(o.values[k])  // value
buf.WriteByte('}')
```

输出 JSON 字段顺序 = `o.keys` 的顺序。

---

## 五、约束汇总

| 约束 | 说明 |
|------|------|
| 用途 | 仅用于 `/healthz` 端点，保证 JSON key 排序输出 |
| 并发 | 非线程安全（无锁） |
| HTML 转义 | 默认开启 `escapeHTML=true`（`<`, `>`, `&` 会被转义） |
| Unmarshal | 按 JSON 输入顺序保持 keys，嵌套对象递归处理 |
| Sort | `SortKeys` 和 `Sort` 均不排序 values map，仅改变 keys 顺序 |