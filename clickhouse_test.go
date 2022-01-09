package clickhouse_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/ClickHouse/clickhouse-go/external"
	"github.com/stretchr/testify/assert"
)

func TestConn(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"127.0.0.1:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: true,
	})
	if assert.NoError(t, err) {
		if err := conn.Ping(context.Background()); assert.NoError(t, err) {
			if assert.NoError(t, conn.Close()) {
				t.Log(conn.Stats())
				t.Log(conn.ServerVersion())
				t.Log(conn.Ping(context.Background()))
			}
		}
	}
}

func TestConnFailover(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{
			"127.0.0.1:9001",
			"127.0.0.1:9002",
			"127.0.0.1:9000",
		},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: true,
	})
	if assert.NoError(t, err) {
		if err := conn.Ping(context.Background()); assert.NoError(t, err) {
			t.Log(conn.ServerVersion())
			t.Log(conn.Ping(context.Background()))
		}
	}
}

func TestPingDeadline(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"127.0.0.1:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: true,
	})
	if assert.NoError(t, err) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		if err := conn.Ping(ctx); assert.Error(t, err) {
			assert.Equal(t, err, context.DeadlineExceeded)
		}
	}
}

func TestExec(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"127.0.0.1:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: true,
	})

	if assert.NoError(t, err) {
		ctx := context.Background()
		conn.Exec(ctx, "DROP TABLE IF EXISTS test_exec")
		conn.Exec(ctx, `
		CREATE TABLE test_exec (
			Column1 UInt8
		) Engine = Memory
		`)

		conn.Exec(ctx, `INSERT INTO test_exec (Column1)
			SELECT 1 FROM system.numbers LIMIT 200
		`)
		assert.NoError(t, conn.Close())
	}
}
func TestContext(t *testing.T) {
	clickhouse.Context(context.Background(),
		clickhouse.WithProgress(func(p *clickhouse.Progress) {}),
		clickhouse.WithSettings(clickhouse.Settings{
			"max_execution_time": 256,
		}),
	)
}

func TestQuery(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"127.0.0.1:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: true,
	})
	if assert.NoError(t, err) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
			"max_block_size": 3,
		}))
		rows, err := conn.Query(ctx, `
			SELECT
				number AS int
				, number::Nullable(UInt64) AS nullable
			FROM system.numbers
			LIMIT 20`)
		if assert.NoError(t, err) {
			if assert.Equal(t, []string{"int", "nullable"}, rows.Columns()) {
				for rows.Next() {
					var (
						rowInt uint64
						rowNil *uint64
					)
					if err := rows.Scan(&rowInt, &rowNil); assert.NoError(t, err) {
						//	t.Log("SCANN", rowInt, rowNil)
					}
				}
				if assert.NoError(t, rows.Close()) {
					assert.NoError(t, rows.Err())
				}
			}
		}
	}
}

func TestQueryBindNumeric(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"127.0.0.1:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: true,
	})
	if assert.NoError(t, err) {
		rows, err := conn.Query(context.Background(), `
		SELECT
			  $1::Int8
			, $2::Int64
			, $1::UInt8
			, $2::UInt64
		`, 10, 1000)
		if assert.NoError(t, err) {
			for rows.Next() {
				var (
					int8Column   int8
					int64Column  int64
					uint8Column  uint8
					uint64Column uint64
				)
				err := rows.Scan(
					&int8Column,
					&int64Column,
					&uint8Column,
					&uint64Column,
				)
				if assert.NoError(t, err) {
					assert.Equal(t, int8(10), int8Column)
					assert.Equal(t, int64(1000), int64Column)
					assert.Equal(t, uint8(10), uint8Column)
					assert.Equal(t, uint64(1000), uint64Column)
				}
			}
		}
	}
}

func TestExternalTable(t *testing.T) {
	table1, err := external.NewTable("external_table_1",
		external.Column("col1", "UInt8"),
		external.Column("col2", "String"),
		external.Column("col3", "DateTime"),
	)
	if assert.NoError(t, err) {
		for i := 0; i < 10; i++ {
			assert.NoError(t, table1.Append(uint8(i), fmt.Sprintf("value_%d", i), time.Now()))
		}
	}
	table2, err := external.NewTable("external_table_2",
		external.Column("col1", "UInt8"),
		external.Column("col2", "String"),
		external.Column("col3", "DateTime"),
	)
	if assert.NoError(t, err) {
		for i := 0; i < 10; i++ {
			assert.NoError(t, table2.Append(uint8(i), fmt.Sprintf("value_%d", i), time.Now()))
		}
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"127.0.0.1:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug: true,
	})
	if assert.NoError(t, err) {
		ctx := clickhouse.Context(context.Background(),
			clickhouse.WithExternalTable(table1, table2),
		)
		if rows, err := conn.Query(ctx, "SELECT * FROM external_table_1"); assert.NoError(t, err) {
			for rows.Next() {
				var (
					col1 uint8
					col2 string
					col3 time.Time
				)
				if err := rows.Scan(&col1, &col2, &col3); assert.NoError(t, err) {
					t.Logf("row: col1=%d, col2=%s, col3=%s\n", col1, col2, col3)
				}
			}
			rows.Close()
		}
		var count uint64
		if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM external_table_1").Scan(&count); assert.NoError(t, err) {
			assert.Equal(t, uint64(10), count)
		}
		if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM external_table_2").Scan(&count); assert.NoError(t, err) {
			assert.Equal(t, uint64(10), count)
		}
		if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM (SELECT * FROM external_table_1 UNION ALL SELECT * FROM external_table_2)").Scan(&count); assert.NoError(t, err) {
			assert.Equal(t, uint64(20), count)
		}
	}
}
