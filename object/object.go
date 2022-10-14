// ソースコード中で整数リテラルに出会うたびに、まずそれをast.IntegerLiteralに変換する。そして、そのASTノードを評価する際に、object.Integerへと変換する。この構造体の中に値を保持しておいて、この構造体への参照を引き回す。

package object

import (
	"fmt"
)

type ObjectType string

const (
	INTEGER_OBJ = "INTEGER"
)

// Monkeyソースコードを評価する際に出てくる値全てをObjectで表現する。全ての値はObjectインターフェースを満たす構造体にラップされる
// インターフェースにする理由は、それぞれの値が異なった内部表現を持つ必要があるため。1つの構造体のフィールドに整数と真偽値を押し込めようとするより、2つの別々の構造体を定義する方が簡単だから
type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }

func (i *Integer) Inspect() string { return fmt.Sprintf("%d", i.Value) }
