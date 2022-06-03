package inverseschema

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

func NewPostgresAdapter(db *sql.DB, schemaname string) *PostgresAdapter {
	return &PostgresAdapter{db: db, schemaname: schemaname}
}

type PostgresAdapter struct {
	db         *sql.DB
	schemaname string
}

var postgresDatatypemap = map[string]Datatype{
	"USER-DEFINED":                DatatypeUserdefined,
	"ARRAY":                       DatatypeArray,
	"boolean":                     DatatypeBoolean,
	"integer":                     DatatypeInt,
	"bigint":                      DatatypeBigint,
	"numeric":                     DatatypeNumeric,
	"text":                        DatatypeText,
	"character varying":           DatatypeVarchar,
	"jsonb":                       DatatypeJsonb,
	"uuid":                        DatatypeUuid,
	"date":                        DatatypeDate,
	"timestamp without time zone": DatatypeTimestamp,
	"timestamp with time zone":    DatatypeTimestampz,
}

func (a *PostgresAdapter) Enums(ctx context.Context) ([]Enum, error) {
	sql := `SELECT 
			t.typname,
			e.enumsortorder as enum_order,
			e.enumlabel as enum_value
		FROM pg_type t 
			JOIN pg_enum e on t.oid = e.enumtypid  
			JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = $1`

	rows, err := a.db.QueryContext(ctx, sql, a.schemaname)
	if err != nil {
		return nil, err
	}
	enumsByName := map[string]Enum{}
	var name string
	var order int
	var label string
	for rows.Next() {
		if err := rows.Scan(&name, &order, &label); err != nil {
			return nil, err
		}
		if enum, ok := enumsByName[name]; ok {
			enum.Values = append(enum.Values, EnumValue{
				Label: label,
				Order: order,
			})
			enumsByName[name] = enum
			continue
		}
		enumsByName[name] = Enum{
			Name: name,
			Values: []EnumValue{{
				Label: label,
				Order: order,
			}},
		}
	}

	enums := make([]Enum, len(enumsByName))
	idx := 0
	for _, enum := range enumsByName {
		enums[idx] = enum
		idx++
	}
	return enums, nil
}

func (a *PostgresAdapter) Tables(ctx context.Context) ([]Table, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname=$1", a.schemaname)
	if err != nil {
		return nil, err
	}
	tables := []Table{}
	for rows.Next() {
		var tablename *string
		if err := rows.Scan(&tablename); err != nil {
			return nil, err
		}
		table, err := a.parseTable(ctx, *tablename)
		if err != nil {
			return nil, err
		}

		tables = append(tables, *table)
	}
	return tables, nil
}

func (a *PostgresAdapter) parseTable(ctx context.Context, tablename string) (*Table, error) {
	table := &Table{
		Name:    tablename,
		Columns: []Column{},
	}

	cols, err := a.parseTableColumns(ctx, tablename)
	if err != nil {
		return table, err
	}
	table.ColumnsByName = make(map[string]Column, len(cols))
	for _, col := range cols {
		table.ColumnsByName[col.Name] = col
	}

	constraints, err := a.parseTableConstraints(ctx, tablename)
	if err != nil {
		return table, err
	}

	if err := a.refrenceConstraints(ctx, table, constraints); err != nil {
		return nil, err
	}

	for _, col := range table.ColumnsByName {
		table.Columns = append(table.Columns, col)
	}
	sort.Slice(table.Columns, func(i, j int) bool {
		return table.Columns[i].OrdinalPosition < table.Columns[j].OrdinalPosition
	})
	return table, nil
}

func (a *PostgresAdapter) refrenceConstraints(ctx context.Context, table *Table, constraints []Constraint) error {
	for _, c := range constraints {
		col, ok := table.ColumnsByName[c.Columnname]
		if !ok {
			continue // how?
		}
		if col.Constraints == nil {
			col.Constraints = []Constraint{c}
		} else {
			col.Constraints = append(col.Constraints, c)
		}

		switch c.Type {
		case ConstraintTypePrimaryKey:
			col.IsPrimary = true
		case ConstraintTypeForeignKey:
			col.IsReference = true
			col.ForeignTablename = c.ForeignTablename
			col.ForeignColumnname = c.ForeignColumnname
		case ConstraintTypeUnique:
			// should we mark as unique if there is more than one column for this index?
			col.IsUnique = true
		}

		table.ColumnsByName[c.Columnname] = col
	}

	return nil

}

func (a *PostgresAdapter) parseTableColumns(ctx context.Context, tablename string) ([]Column, error) {
	sql := `SELECT 
		c.ordinal_position,
		c.column_name,
		c.column_default,
		c.is_nullable,
		c.data_type,
		e.data_type AS element_type,
		e.udt_catalog AS element_udt_catalog,
		e.udt_schema AS element_udt_schema,
		e.udt_name AS element_udt_name,
		c.character_maximum_length,
		c.numeric_precision,
		c.udt_catalog,
		c.udt_schema,
		c.udt_name,
		(SELECT pg_catalog.col_description(oid,c.ordinal_position::int) from pg_catalog.pg_class pc where pc.relname=c.table_name) as column_comment
		FROM information_schema.columns c
		LEFT JOIN information_schema.element_types e ON ((c.table_catalog, c.table_schema, c.table_name, 'TABLE', c.dtd_identifier)
		= (e.object_catalog, e.object_schema, e.object_name, e.object_type, e.collection_type_identifier))
		WHERE c.table_schema=$1 AND c.table_name=$2`

	rows, err := a.db.QueryContext(ctx, sql, a.schemaname, tablename)
	if err != nil {
		return nil, err
	}
	cols := []Column{}
	for rows.Next() {
		var ordinalPosition int
		var columnName string
		var columnDefault *string
		var isNullable *string
		var datatypeRaw string
		var elementArraytypeRaw *string
		var elementUdtCatalog *string
		var elementUdtSchema *string
		var elementUdtName *string
		var characterMaximumLength *int
		var numericPrecision *int
		var udtCatalog *string
		var udtSchema *string
		var udtName *string
		var comments *string

		if err := rows.Scan(
			&ordinalPosition,
			&columnName,
			&columnDefault,
			&isNullable,
			&datatypeRaw,
			&elementArraytypeRaw,
			&elementUdtCatalog,
			&elementUdtSchema,
			&elementUdtName,
			&characterMaximumLength,
			&numericPrecision,
			&udtCatalog,
			&udtSchema,
			&udtName,
			&comments,
		); err != nil {
			return nil, err
		}

		col := Column{
			OrdinalPosition: ordinalPosition,
			Name:            columnName,
			DatatypeRaw:     datatypeRaw,
		}

		if comments != nil {
			col.Comments = *comments
		}
		datatype, ok := postgresDatatypemap[datatypeRaw]
		if ok {
			col.Datatype = datatype
		} else {
			col.Datatype = DatatypeUnknown
		}
		if characterMaximumLength != nil {
			col.CharacterMaxLength = *characterMaximumLength
		}

		if isNullable != nil && *isNullable == "YES" {
			col.IsNullable = true
		}
		if columnDefault != nil && len(*columnDefault) > 0 {
			col.HasDefault = true
			col.Default = *columnDefault
		}
		if col.Datatype == DatatypeUserdefined {
			col.IsUserDefined = true
			col.UserDefinedType = &UserDefinedType{
				Name:   *udtName,
				Schema: *udtSchema,
			}
		}
		// case injection for datatype array
		if col.Datatype == DatatypeArray {
			col.IsArray = true
			elementDatatype, ok := postgresDatatypemap[*elementArraytypeRaw]
			if ok {
				col.Datatype = elementDatatype
			} else {
				col.Datatype = DatatypeUnknown
			}
			if col.Datatype == DatatypeUserdefined {
				col.IsUserDefined = true
				col.UserDefinedType = &UserDefinedType{
					Name:   *elementUdtName,
					Schema: *elementUdtSchema,
				}
			}
		}
		cols = append(cols, col)
	}
	return cols, nil
}

func (a *PostgresAdapter) parseTableConstraints(ctx context.Context, tablename string) ([]Constraint, error) {
	sql := `SELECT
		tc.constraint_name, tc.constraint_type, kcu.column_name, 
		ccu.table_name AS foreign_table_name,
		ccu.column_name AS foreign_column_name 
	FROM information_schema.table_constraints AS tc 
		LEFT JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
		LEFT JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
	WHERE tc.table_schema=$1 AND tc.table_name=$2 AND tc.constraint_type IN ('PRIMARY KEY', 'FOREIGN KEY', 'UNIQUE')`

	rows, err := a.db.QueryContext(ctx, sql, a.schemaname, tablename)
	if err != nil {
		return nil, err
	}
	constraints := []Constraint{}
	for rows.Next() {
		var constraintname string
		var constrainttype string
		var columnname *string
		var foreignTablename *string
		var foreignColumnname *string

		if err := rows.Scan(
			&constraintname,
			&constrainttype,
			&columnname,
			&foreignTablename,
			&foreignColumnname,
		); err != nil {
			return nil, err
		}

		c := Constraint{
			Name:              constraintname,
			Tablename:         tablename,
			Columnname:        *columnname,
			ForeignTablename:  *foreignTablename,
			ForeignColumnname: *foreignColumnname,
		}
		switch constrainttype {
		case "PRIMARY KEY":
			c.Type = ConstraintTypePrimaryKey
		case "FOREIGN KEY":
			c.Type = ConstraintTypeForeignKey
		case "UNIQUE":
			c.Type = ConstraintTypeUnique
		default:
			return nil, fmt.Errorf("unsupported constraint type: %s", constrainttype)
		}
		constraints = append(constraints, c)

	}
	return constraints, nil
}
