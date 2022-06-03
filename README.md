# inverseschema

A simple zero dependency package for scanning a database schema into structs 

## Why does this exists

There are a few options out there that generate strongly typed models out of database schema, they are all however relatively opinionated and expect usage through CLI, this package allows you to use it within your own code generation process, all it does is run a few queries and converts the result into golang types


## Installation

go get github.com/oiime/inverseschema


## Usage example

```golang
package main

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/oiime/inverseschema"
)

func main() {
	db, err := sql.Open("postgres", "postgres://foo:bar@localhost:5435/foobar?sslmode=disable")
	if err != nil {
		panic(err)
	}
	schema := inverseschema.NewSchema(inverseschema.NewPostgresAdapter(db, "public"))

	if err := schema.Parse(); err != nil {
		panic(err)
	}
    
    // tables and enums are now present within `schema`
    // schema.Tables
    // schema.Enums
}

```

### Result Type

```golang

type Schema struct {
	Tables  []Table
	Enums   []Enum
}

type Table struct {
	Name          string            `json:"name,omitempty"`
	Columns       []Column          `json:"columns,omitempty"`
	ColumnsByName map[string]Column `json:"columns_by_name,omitempty"`
}

type Constraint struct {
	Name              string         `json:"name,omitempty"`
	Type              ConstraintType `json:"type,omitempty"`
	Tablename         string         `json:"tablename,omitempty"`
	Columnname        string         `json:"columnname,omitempty"`
	ForeignTablename  string         `json:"foreign_tablename,omitempty"`
	ForeignColumnname string         `json:"foreign_columnname,omitempty"`
}

type UserDefinedType struct {
	Name   string `json:"name,omitempty"`
	Schema string `json:"schema,omitempty"`
}

type Column struct {
	OrdinalPosition    int              `json:"ordinal_position,omitempty"`
	Name               string           `json:"name,omitempty"`
	Constraints        []Constraint     `json:"constraints,omitempty"`
	IsReference        bool             `json:"is_reference,omitempty"`
	ForeignTablename   string           `json:"foreign_tablename,omitempty"`
	ForeignColumnname  string           `json:"foreign_columnname,omitempty"`
	IsPrimary          bool             `json:"is_primary,omitempty"`
	IsUnique           bool             `json:"is_unique,omitempty"`
	HasDefault         bool             `json:"has_default,omitempty"`
	Default            string           `json:"default,omitempty"`
	IsNullable         bool             `json:"is_nullable,omitempty"`
	DatatypeRaw        string           `json:"datatype_raw,omitempty"`
	Datatype           Datatype         `json:"datatype,omitempty"`
	IsUserDefined      bool             `json:"is_user_defined,omitempty"`
	IsArray            bool             `json:"is_array,omitempty"`
	CharacterMaxLength int              `json:"character_max_length,omitempty"`
	UserDefinedType    *UserDefinedType `json:"user_defined_type,omitempty"`
	Comments           string           `json:"comments,omitempty"`
}

type Enum struct {
	Name   string      `json:"name,omitempty"`
	Values []EnumValue `json:"values,omitempty"`
}

type EnumValue struct {
	Label string `json:"label,omitempty"`
	Order int    `json:"order,omitempty"`
}

type Datatype int

const (
	DatatypeUnknown Datatype = iota
	DatatypeUserdefined
	DatatypeArray
	DatatypeBigint
	DatatypeInt
	DatatypeSmallint
	DatatypeDecimal
	DatatypeNumeric
	DatatypeVariableNumeric
	DatatypeJsonb
	DatatypeJson
	DatatypeText
	DatatypeVarchar
	DatatypeBoolean
	DatatypeDate
	DatatypeTimestamp
	DatatypeTimestampz
	DatatypeUuid
)

```
