package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var (
	regRealDataType = regexp.MustCompile(`[^\d](\d+)[^\d]?`)
	regFullDataType = regexp.MustCompile(`[^\d]*(\d+)[^\d]?`)
	defaultMigrationsFolder = "migrations/sql"
)

// Migrator m struct
type Migrator struct {
	Config
	Models        []interface{}
	migrationPath string
}

// Config schema config
type Config struct {
	CreateIndexAfterCreateTable bool
	DB                          *gorm.DB
	gorm.Dialector
}

// GormDataTypeInterface gorm data type interface
type GormDataTypeInterface interface {
	GormDBDataType(*gorm.DB, *schema.Field) string
}

func New(db *gorm.DB, migrationFolder ...string) *Migrator {
	migrationPath := defaultMigrationsFolder
	if len(migrationFolder) > 0 {
		migrationPath = migrationFolder[0]
	}
	return &Migrator{
		Config: Config{
			CreateIndexAfterCreateTable: true,
			DB:                          db,
		},
		Models:        make([]interface{}, 0),
		migrationPath: migrationPath,
	}
}

// RegisterModel run migration with statement value
func (m *Migrator) RegisterModel(values ...interface{}) {
	m.Models = append(m.Models, values...)
}

// RunWithValue run migration with statement value
func (m *Migrator) RunWithValue(value interface{}, fc func(*gorm.Statement) error) error {
	stmt := &gorm.Statement{DB: m.DB}
	if m.DB.Statement != nil {
		stmt.Table = m.DB.Statement.Table
		stmt.TableExpr = m.DB.Statement.TableExpr
	}

	if table, ok := value.(string); ok {
		stmt.Table = table
	} else if err := stmt.ParseWithSpecialTableName(value, stmt.Table); err != nil {
		return err
	}

	return fc(stmt)
}

// AutoMigrate auto migrate values
func (m *Migrator) AutoMigrate() (string, string, error) {
	var migrationSQLUp string
	var migrationSQLUpDown string
	taps := "\n"
	excludedTables := m.ExcludedTable(m.Models)
	if len(excludedTables) > 0 {
		for _, tableName := range excludedTables {
			if tableName == "schema_migrations" {
				continue
			}
			migrationSQLUpDown += "-- Drop Table \n"
			migrationSQLUpDown += fmt.Sprintf("DROP TABLE IF EXISTS %s\n", tableName)
		}
	}

	for _, value := range m.ReorderModels(m.Models, true) {
		if !m.HasTable(value) {
			migrationSQLUp_, migrationSQLUpDown_ := m.CreateTable(value)
			migrationSQLUp += migrationSQLUp_ + taps
			migrationSQLUpDown += migrationSQLUpDown_ + taps
		} else {
			var alterSchemaSQL string
			var revertAlterSchemaSQL string
			if err := m.RunWithValue(value, func(stmt *gorm.Statement) (errr error) {
				columnTypes, err := m.ColumnTypes(value)
				if err != nil {
					return err
				}
				removedColumnMap := map[string]bool{}
				foundColumnMap := map[string]bool{}
				for _, dbName := range stmt.Schema.DBNames {
					field := stmt.Schema.FieldsByDBName[dbName]
					var foundColumn gorm.ColumnType

					for _, columnType := range columnTypes {
						columnTypeName := columnType.Name()
						if columnTypeName == dbName {
							foundColumn = columnType
							break
						}
					}

					if foundColumn == nil {
						// not found, add column
						foundColumnMap[dbName] = true
						alterSchemaSQL += m.AddColumn(value, dbName)
						revertAlterSchemaSQL += m.DropColumn(stmt, dbName)
					} else {
						alterSchemaSQL += m.MigrateColumn(value, field, foundColumn, stmt)
						// found, smart migrate
					}

				}
				// check for column that have been removed from model and remove them in table
				for _, columnType := range columnTypes {
					columnTypeName := columnType.Name()
					_, foundInFieldsByDBName := stmt.Schema.FieldsByDBName[columnTypeName]
					_, removedColumn := removedColumnMap[columnTypeName]
					if !foundInFieldsByDBName && !removedColumn {
						migrationSQLUp += m.DropColumn(stmt, columnTypeName)
						// TODO: add the sql type
						migrationSQLUpDown += buildRawSQL(m.DB, "ALTER TABLE ? ADD ?", []interface{}{m.CurrentTable(stmt), clause.Column{Name: columnTypeName}}...)
						// make it has removed
						removedColumnMap[columnTypeName] = true
					}
				}

				for _, idx := range stmt.Schema.ParseIndexes() {
					if !m.HasIndex(value, idx.Name) {
						createIndexSQLRaw_, downIndexSQLRaw_ := m.CreateIndex(value, idx.Name)
						alterSchemaSQL += createIndexSQLRaw_
						if _, found := foundColumnMap[idx.Fields[0].DBName]; !found {
							revertAlterSchemaSQL += downIndexSQLRaw_
						}
					}
				}

				return nil
			}); err != nil {
				return "", "", err
			}

			migrationSQLUp += alterSchemaSQL + taps
			migrationSQLUpDown += revertAlterSchemaSQL + taps
		}
	}

	if strings.TrimSpace(migrationSQLUp) != "" || strings.TrimSpace(migrationSQLUpDown) != "" {
		migrationSQLUp = "BEGIN;\n\n" + migrationSQLUp + "\nCOMMIT;"
		migrationSQLUpDown = "BEGIN;\n\n" + migrationSQLUpDown + "\nCOMMIT;"
		return migrationSQLUp, migrationSQLUpDown, nil
	}

	return "", "", nil
}

// CreateTable create table in database for values
func (m *Migrator) CreateTable(values ...interface{}) (string, string) {
	var createTableSQLRaw string
	var dropTableSQLRaw string
	for _, value := range m.ReorderModels(values, false) {
		if err := m.RunWithValue(value, func(stmt *gorm.Statement) (errr error) {
			var (
				createTableSQL          = "CREATE TABLE ? ("
				dropTableSQL            = "DROP TABLE IF EXISTS ?"
				values                  = []interface{}{m.CurrentTable(stmt)}
				hasPrimaryKeyInDataType bool
			)

			//Add comment
			createTableSQL = "-- Create Table \n" + createTableSQL
			dropTableSQL = "-- Drop Table \n" + dropTableSQL

			for _, dbName := range stmt.Schema.DBNames {
				field := stmt.Schema.FieldsByDBName[dbName]
				if !field.IgnoreMigration {
					createTableSQL += "? ?"
					hasPrimaryKeyInDataType = hasPrimaryKeyInDataType || strings.Contains(strings.ToUpper(string(field.DataType)), "PRIMARY KEY")
					values = append(values, clause.Column{Name: dbName}, m.DB.Migrator().FullDataTypeOf(field))
					createTableSQL += ","
				}
			}

			if !hasPrimaryKeyInDataType && len(stmt.Schema.PrimaryFields) > 0 {
				createTableSQL += "PRIMARY KEY ?,"
				primaryKeys := []interface{}{}
				for _, field := range stmt.Schema.PrimaryFields {
					primaryKeys = append(primaryKeys, clause.Column{Name: field.DBName})
				}

				values = append(values, primaryKeys)
			}

			for _, idx := range stmt.Schema.ParseIndexes() {
				if m.CreateIndexAfterCreateTable {
					defer func(value interface{}, name string) {
						if errr == nil {
							createIndexSQLRaw_, _ := m.CreateIndex(value, name)
							createTableSQLRaw += createIndexSQLRaw_
						}
					}(value, idx.Name)
				} else {
					if idx.Class != "" {
						createTableSQL += idx.Class + " "
					}
					createTableSQL += "INDEX ? ?"

					if idx.Comment != "" {
						createTableSQL += fmt.Sprintf(" COMMENT '%s'", idx.Comment)
					}

					if idx.Option != "" {
						createTableSQL += " " + idx.Option
					}

					createTableSQL += ","
					values = append(values, clause.Column{Name: idx.Name}, m.DB.Migrator().(BuildIndexOptionsInterface).BuildIndexOptions(idx.Fields, stmt))
				}
			}

			for _, rel := range stmt.Schema.Relationships.Relations {
				if !m.DB.DisableForeignKeyConstraintWhenMigrating {
					if constraint := rel.ParseConstraint(); constraint != nil {
						if constraint.Schema == stmt.Schema {
							sql, vars := buildConstraint(constraint)
							createTableSQL += sql + ","
							values = append(values, vars...)
						}
					}
				}
			}

			for _, chk := range stmt.Schema.ParseCheckConstraints() {
				createTableSQL += "CONSTRAINT ? CHECK (?),"
				values = append(values, clause.Column{Name: chk.Name}, clause.Expr{SQL: chk.Constraint})
			}

			createTableSQL = strings.TrimSuffix(createTableSQL, ",")

			createTableSQL += ")"

			if tableOption, ok := m.DB.Get("gorm:table_options"); ok {
				createTableSQL += fmt.Sprint(tableOption)
			}

			createTableSQLRaw += buildRawSQL(m.DB, createTableSQL, values...)
			dropTableSQLRaw += buildRawSQL(m.DB, dropTableSQL, values...)
			_ = createTableSQLRaw // TODO do something with this

			//TODO: remove this
			return nil
		}); err != nil {
			return "", ""
		}
	}

	return createTableSQLRaw, dropTableSQLRaw
}

// HasTable returns table exists or not for value, value could be a struct or string
func (m *Migrator) HasTable(value interface{}) bool {
	var count int64
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		currentSchema, curTable := m.CurrentSchema(stmt, stmt.Table)
		return m.DB.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ? AND table_type = ?", currentSchema, curTable, "BASE TABLE").Scan(&count).Error
	})
	return count > 0
}

// ExcludedTable returns list of excluded tables in the database.
func (m *Migrator) ExcludedTable(values []interface{}) []string {
	type Tables struct {
		TableName string
	}

	var currentTables []Tables
	var excludedTables []string
	m.RunWithValue(values[0], func(stmt *gorm.Statement) error {
		currentSchema, _ := m.CurrentSchema(stmt, stmt.Table)
		return m.DB.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = ?", currentSchema, "BASE TABLE").Scan(&currentTables).Error
	})

	tableName := map[string]bool{}
	stmt := &gorm.Statement{DB: m.DB}
	for i := len(values) - 1; i >= 0; i-- {
		if table, ok := values[i].(string); ok {
			tableName[table] = true
		} else {
			if err := stmt.ParseWithSpecialTableName(values[i], ""); err != nil {
				return nil
			}
			tableName[stmt.Table] = true
			stmt.Table = ""
		}
	}

	for _, val := range currentTables {
		if _, ok := tableName[val.TableName]; !ok {
			excludedTables = append(excludedTables, val.TableName)
		}
	}

	return excludedTables
}

func (m *Migrator) CurrentSchema(stmt *gorm.Statement, table string) (interface{}, interface{}) {
	if strings.Contains(table, ".") {
		if tables := strings.Split(table, `.`); len(tables) == 2 {
			return tables[0], tables[1]
		}
	}

	if stmt.TableExpr != nil {
		if tables := strings.Split(stmt.TableExpr.SQL, `"."`); len(tables) == 2 {
			return strings.TrimPrefix(tables[0], `"`), table
		}
	}
	return clause.Expr{SQL: "CURRENT_SCHEMA()"}, table
}

// AddColumn create `name` column for value
func (m *Migrator) AddColumn(value interface{}, name string) string {
	var addColumnRawSQL string
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		// avoid using the same name field
		f := stmt.Schema.LookUpField(name)
		if f == nil {
			return fmt.Errorf("failed to look up field with name: %s", name)
		}

		if !f.IgnoreMigration {
			addColumnRawSQL = buildRawSQL(m.DB, "ALTER TABLE ? ADD ? ?", []interface{}{m.CurrentTable(stmt), clause.Column{Name: f.DBName}, m.DB.Migrator().FullDataTypeOf(f)}...)
		}

		return nil
	})
	return addColumnRawSQL
}

// DropColumn drop value's `name` column
func (m *Migrator) DropColumn(stmt *gorm.Statement, name string) string {
	var dropColumnRawSQL string

	if field := stmt.Schema.LookUpField(name); field != nil {
		name = field.DBName
	}
	dropColumnRawSQL = buildRawSQL(m.DB, "ALTER TABLE ? DROP COLUMN ?", []interface{}{m.CurrentTable(stmt), clause.Column{Name: name}}...)

	return dropColumnRawSQL
}

// AlterColumn alter value's `field` column' type based on schema definition
func (m *Migrator) AlterColumn(value interface{}, field string) string {
	var alterColumnRawSQL string
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		if field := stmt.Schema.LookUpField(field); field != nil {
			fileType := m.DB.Migrator().FullDataTypeOf(field)
			alterColumnRawSQL = buildRawSQL(m.DB, "ALTER TABLE ? ALTER COLUMN ? TYPE ?", []interface{}{m.CurrentTable(stmt), clause.Column{Name: field.DBName}, fileType}...)
		}
		return fmt.Errorf("failed to look up field with name: %s", field)
	})
	return alterColumnRawSQL
}

// HasColumn check has column `field` for value or not
func (m *Migrator) HasColumn(value interface{}, field string) bool {
	var count int64
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		currentDatabase := m.DB.Migrator().CurrentDatabase()
		name := field
		if field := stmt.Schema.LookUpField(field); field != nil {
			name = field.DBName
		}

		return m.DB.Raw(
			"SELECT count(*) FROM INFORMATION_SCHEMA.columns WHERE table_schema = ? AND table_name = ? AND column_name = ?",
			currentDatabase, stmt.Table, name,
		).Row().Scan(&count)
	})

	return count > 0
}

// MigrateColumn migrate column
func (m *Migrator) MigrateColumn(value interface{}, field *schema.Field, columnType gorm.ColumnType, stmt *gorm.Statement) string {
	// found, smart migrate
	fullDataType := strings.ToLower(m.DB.Migrator().FullDataTypeOf(field).SQL)
	realDataType := strings.ToLower(columnType.DatabaseTypeName())

	alterColumn := false

	// check size
	if length, ok := columnType.Length(); length != int64(field.Size) {
		if length > 0 && field.Size > 0 {
			alterColumn = true
		} else {
			// has size in data type and not equal
			// Since the following code is frequently called in the for loop, reg optimization is needed here
			matches := regRealDataType.FindAllStringSubmatch(realDataType, -1)
			matches2 := regFullDataType.FindAllStringSubmatch(fullDataType, -1)
			if (len(matches) == 1 && matches[0][1] != fmt.Sprint(field.Size) || !field.PrimaryKey) &&
				(len(matches2) == 1 && matches2[0][1] != fmt.Sprint(length) && ok) {
				alterColumn = true
			}
		}
	}

	// check precision
	if precision, _, ok := columnType.DecimalSize(); ok && int64(field.Precision) != precision {
		if regexp.MustCompile(fmt.Sprintf("[^0-9]%d[^0-9]", field.Precision)).MatchString(m.DataTypeOf(field)) {
			alterColumn = true
		}
	}

	// check nullable
	if nullable, ok := columnType.Nullable(); ok && nullable == field.NotNull {
		// not primary key & database is nullable
		if !field.PrimaryKey && nullable {
			alterColumn = true
		}
	}

	// check unique
	if unique, ok := columnType.Unique(); ok && unique != field.Unique {
		// not primary key
		if !field.PrimaryKey {
			alterColumn = true
		}
	}

	// check default value
	if v, ok := columnType.DefaultValue(); ok && v != field.DefaultValue {
		// not primary key
		if !field.PrimaryKey {
			alterColumn = true
		}
	}

	// check comment
	if comment, ok := columnType.Comment(); ok && comment != field.Comment {
		// not primary key
		if !field.PrimaryKey {
			alterColumn = true
		}
	}

	if alterColumn && !field.IgnoreMigration {
		return m.AlterColumn(value, field.Name)
	}

	return ""
}

// ColumnTypes return columnTypes []gorm.ColumnType and execErr error
// TODO: rewrite this function
func (m *Migrator) ColumnTypes(value interface{}) ([]ColumnType, error) {
	columnTypes := make([]ColumnType, 0)
	execErr := m.RunWithValue(value, func(stmt *gorm.Statement) (err error) {
		rows, err := m.DB.Session(&gorm.Session{}).Table(stmt.Table).Rows()
		if err != nil {
			return err
		}

		defer func() {
			err = rows.Close()
		}()

		var rawColumnTypes []*sql.ColumnType
		rawColumnTypes, err = rows.ColumnTypes()
		if err != nil {
			return err
		}

		for _, c := range rawColumnTypes {
			columnTypes = append(columnTypes, ColumnType{SQLColumnType: c})
		}

		return
	})

	return columnTypes, execErr
}

// CreateView create view
func (m *Migrator) CreateView(name string, option gorm.ViewOption) error {
	return gorm.ErrNotImplemented
}

// DropView drop view
func (m *Migrator) DropView(name string) error {
	return gorm.ErrNotImplemented
}

func buildConstraint(constraint *schema.Constraint) (sql string, results []interface{}) {
	sql = "CONSTRAINT ? FOREIGN KEY ? REFERENCES ??"
	if constraint.OnDelete != "" {
		sql += " ON DELETE " + constraint.OnDelete
	}

	if constraint.OnUpdate != "" {
		sql += " ON UPDATE " + constraint.OnUpdate
	}

	var foreignKeys, references []interface{}
	for _, field := range constraint.ForeignKeys {
		foreignKeys = append(foreignKeys, clause.Column{Name: field.DBName})
	}

	for _, field := range constraint.References {
		references = append(references, clause.Column{Name: field.DBName})
	}
	results = append(results, clause.Table{Name: constraint.Name}, foreignKeys, clause.Table{Name: constraint.ReferenceSchema.Table}, references)
	return
}

// GuessConstraintAndTable guess statement's constraint and it's table based on name
func (m *Migrator) GuessConstraintAndTable(stmt *gorm.Statement, name string) (_ *schema.Constraint, _ *schema.Check, table string) {
	if stmt.Schema == nil {
		return nil, nil, stmt.Table
	}

	checkConstraints := stmt.Schema.ParseCheckConstraints()
	if chk, ok := checkConstraints[name]; ok {
		return nil, &chk, stmt.Table
	}

	getTable := func(rel *schema.Relationship) string {
		switch rel.Type {
		case schema.HasOne, schema.HasMany:
			return rel.FieldSchema.Table
		case schema.Many2Many:
			return rel.JoinTable.Table
		}
		return stmt.Table
	}

	for _, rel := range stmt.Schema.Relationships.Relations {
		if constraint := rel.ParseConstraint(); constraint != nil && constraint.Name == name {
			return constraint, nil, getTable(rel)
		}
	}

	if field := stmt.Schema.LookUpField(name); field != nil {
		for k := range checkConstraints {
			if checkConstraints[k].Field == field {
				v := checkConstraints[k]
				return nil, &v, stmt.Table
			}
		}

		for _, rel := range stmt.Schema.Relationships.Relations {
			if constraint := rel.ParseConstraint(); constraint != nil && rel.Field == field {
				return constraint, nil, getTable(rel)
			}
		}
	}

	return nil, nil, stmt.Schema.Table
}

// BuildIndexOptions build index options
func (m *Migrator) BuildIndexOptions(opts []schema.IndexOption, stmt *gorm.Statement) (results []interface{}) {
	for _, opt := range opts {
		str := stmt.Quote(opt.DBName)
		if opt.Expression != "" {
			str = opt.Expression
		} else if opt.Length > 0 {
			str += fmt.Sprintf("(%d)", opt.Length)
		}

		if opt.Collate != "" {
			str += " COLLATE " + opt.Collate
		}

		if opt.Sort != "" {
			str += " " + opt.Sort
		}
		results = append(results, clause.Expr{SQL: str})
	}
	return
}

// BuildIndexOptionsInterface build index options interface
type BuildIndexOptionsInterface interface {
	BuildIndexOptions([]schema.IndexOption, *gorm.Statement) []interface{}
}

// CreateIndex create index `name`
func (m *Migrator) CreateIndex(value interface{}, name string) (string, string) {
	var createIndexRawSQL string
	var dropIndexRawSQL string
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		if idx := stmt.Schema.LookIndex(name); idx != nil {
			opts := m.BuildIndexOptions(idx.Fields, stmt)
			values := []interface{}{clause.Column{Name: idx.Name}, m.CurrentTable(stmt), opts}

			createIndexSQL := "CREATE "
			dropIndexSQL := "DROP INDEX ?"
			if idx.Class != "" {
				createIndexSQL += idx.Class + " "
			}
			createIndexSQL += "INDEX ? ON ??"

			if idx.Type != "" {
				createIndexSQL += " USING " + idx.Type
			}

			if idx.Comment != "" {
				createIndexSQL += fmt.Sprintf(" COMMENT '%s'", idx.Comment)
			}

			if idx.Option != "" {
				createIndexSQL += " " + idx.Option
			}

			createIndexRawSQL = buildRawSQL(m.DB, createIndexSQL, values...)
			dropIndexRawSQL = buildRawSQL(m.DB, dropIndexSQL, values...)
		}
		return nil
	})
	return createIndexRawSQL, dropIndexRawSQL
}

// HasIndex check has index `name` or not
func (m *Migrator) HasIndex(value interface{}, name string) bool {
	var count int64
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		if idx := stmt.Schema.LookIndex(name); idx != nil {
			name = idx.Name
		}
		currentSchema, curTable := m.CurrentSchema(stmt, stmt.Table)
		return m.DB.Raw(
			"SELECT count(*) FROM pg_indexes WHERE tablename = ? AND indexname = ? AND schemaname = ?", curTable, name, currentSchema,
		).Scan(&count).Error
	})

	return count > 0
}

// CurrentDatabase returns current database name
func (m *Migrator) CurrentDatabase() (name string) {
	m.DB.Raw("SELECT DATABASE()").Row().Scan(&name)
	return
}

// ReorderModels reorder models according to constraint dependencies
func (m *Migrator) ReorderModels(values []interface{}, autoAdd bool) (results []interface{}) {
	type Dependency struct {
		*gorm.Statement
		Depends []*schema.Schema
	}

	var (
		modelNames, orderedModelNames []string
		orderedModelNamesMap          = map[string]bool{}
		parsedSchemas                 = map[*schema.Schema]bool{}
		valuesMap                     = map[string]Dependency{}
		insertIntoOrderedList         func(name string)
		parseDependence               func(value interface{}, addToList bool)
	)

	parseDependence = func(value interface{}, addToList bool) {
		dep := Dependency{
			Statement: &gorm.Statement{DB: m.DB, Dest: value},
		}
		beDependedOn := map[*schema.Schema]bool{}
		// support for special table name
		if err := dep.ParseWithSpecialTableName(value, m.DB.Statement.Table); err != nil {
			m.DB.Logger.Error(context.Background(), "failed to parse value %#v, got error %v", value, err)
		}
		if _, ok := parsedSchemas[dep.Statement.Schema]; ok {
			return
		}
		parsedSchemas[dep.Statement.Schema] = true

		for _, rel := range dep.Schema.Relationships.Relations {
			if c := rel.ParseConstraint(); c != nil && c.Schema == dep.Statement.Schema && c.Schema != c.ReferenceSchema {
				dep.Depends = append(dep.Depends, c.ReferenceSchema)
			}

			if rel.Type == schema.HasOne || rel.Type == schema.HasMany {
				beDependedOn[rel.FieldSchema] = true
			}

			if rel.JoinTable != nil {
				// append join value
				defer func(rel *schema.Relationship, joinValue interface{}) {
					if !beDependedOn[rel.FieldSchema] {
						dep.Depends = append(dep.Depends, rel.FieldSchema)
					} else {
						fieldValue := reflect.New(rel.FieldSchema.ModelType).Interface()
						parseDependence(fieldValue, autoAdd)
					}
					parseDependence(joinValue, autoAdd)
				}(rel, reflect.New(rel.JoinTable.ModelType).Interface())
			}
		}

		valuesMap[dep.Schema.Table] = dep

		if addToList {
			modelNames = append(modelNames, dep.Schema.Table)
		}
	}

	insertIntoOrderedList = func(name string) {
		if _, ok := orderedModelNamesMap[name]; ok {
			return // avoid loop
		}
		orderedModelNamesMap[name] = true

		if autoAdd {
			dep := valuesMap[name]
			for _, d := range dep.Depends {
				if _, ok := valuesMap[d.Table]; ok {
					insertIntoOrderedList(d.Table)
				} else {
					parseDependence(reflect.New(d.ModelType).Interface(), autoAdd)
					insertIntoOrderedList(d.Table)
				}
			}
		}

		orderedModelNames = append(orderedModelNames, name)
	}

	for _, value := range values {
		if v, ok := value.(string); ok {
			results = append(results, v)
		} else {
			parseDependence(value, true)
		}
	}

	for _, name := range modelNames {
		insertIntoOrderedList(name)
	}

	for _, name := range orderedModelNames {
		results = append(results, valuesMap[name].Statement.Dest)
	}
	return
}

// DataTypeOf return field's db data type
func (m *Migrator) DataTypeOf(field *schema.Field) string {
	fieldValue := reflect.New(field.IndirectFieldType)
	if dataTyper, ok := fieldValue.Interface().(GormDataTypeInterface); ok {
		if dataType := dataTyper.GormDBDataType(m.DB, field); dataType != "" {
			return dataType
		}
	}
	return m.Dialector.DataTypeOf(field)
}

// CurrentTable returns current statement's table expression
func (m *Migrator) CurrentTable(stmt *gorm.Statement) interface{} {
	if stmt.TableExpr != nil {
		return *stmt.TableExpr
	}
	return clause.Table{Name: stmt.Table}
}

func buildRawSQL(db *gorm.DB, sql string, values ...interface{}) string {
	stmt := &gorm.Statement{DB: db}
	stmt.SQL = strings.Builder{}
	if strings.Contains(sql, "@") {
		clause.NamedExpr{SQL: sql, Vars: values}.Build(stmt)
	} else {
		clause.Expr{SQL: sql, Vars: values}.Build(stmt)
	}

	return stmt.SQL.String() + "; \n"
}
