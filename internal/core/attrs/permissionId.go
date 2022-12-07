package attrs

import (
	"context"
	"fmt"
	"github.com/PGSSoft/terraform-provider-mssql/internal/sql"
	"github.com/hashicorp/terraform-plugin-framework/attr"
)

func PermissionIdType[T sql.NumericObjectId]() attr.Type {
	var t attr.Type
	t = permissionIdType[T]{
		compositeIdType{
			elemCount: 2,
			valueFactory: func(id CompositeId) attr.Value {
				id.attrType = &t
				return PermissionId[T]{id}
			},
		},
	}
	return t
}

type permissionIdType[T sql.NumericObjectId] struct {
	compositeIdType
}

func PermissionIdValue[T sql.NumericObjectId](id T, permission string) PermissionId[T] {
	t := PermissionIdType[T]()

	return PermissionId[T]{
		CompositeId{
			attrType: &t,
			elems:    []string{fmt.Sprint(id), permission},
		},
	}
}

type PermissionId[T sql.NumericObjectId] struct {
	CompositeId
}

func (id PermissionId[T]) ObjectId(ctx context.Context) T {
	return T(id.GetInt(ctx, 0))
}

func (id PermissionId[T]) Permission() string {
	return id.GetString(1)
}
