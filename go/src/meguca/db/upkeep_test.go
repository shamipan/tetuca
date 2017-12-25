package db

import (
	"fmt"
	"meguca/common"
	"meguca/config"
	. "meguca/test"
	"testing"
	"time"
)

const eightDays = time.Hour * 24 * 8

type threadExpiryCases []struct {
	id    uint64
	board string
	time  time.Time
}

func assertDeleted(t *testing.T, q string, del bool) {
	t.Helper()

	q = fmt.Sprintf(`select exists (select 1 %s)`, q)
	var exists bool
	err := db.QueryRow(q).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}

	deleted := !exists
	if deleted != del {
		LogUnexpected(t, del, deleted)
	}
}

func assertBoardDeleted(t *testing.T, id string, del bool) {
	t.Helper()

	q := fmt.Sprintf(`from boards where id = '%s'`, id)
	assertDeleted(t, q, del)
}

func assertThreadDeleted(t *testing.T, id uint64, del bool) {
	t.Helper()

	q := fmt.Sprintf(`from threads where id = '%d'`, id)
	assertDeleted(t, q, del)
}

func TestDeleteUnusedBoards(t *testing.T) {
	assertTableClear(t, "boards")
	config.Set(config.Configs{
		BoardExpiry: 7,
	})

	t.Run("no boards", func(t *testing.T) {
		(*config.Get()).PruneBoards = true

		if err := deleteUnusedBoards(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("board with no threads", testBoardNoThreads)
	t.Run("pruning disabled", testBoardPruningDisabled)
	t.Run("board with threads", testDeleteUnusedBoards)
}

func testBoardNoThreads(t *testing.T) {
	(*config.Get()).PruneBoards = true

	err := WriteBoard(nil, BoardConfigs{
		Created: time.Now().Add(-eightDays),
		BoardConfigs: config.BoardConfigs{
			ID:        "l",
			Eightball: []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteUnusedBoards(); err != nil {
		t.Fatal(err)
	}
	assertBoardDeleted(t, "l", true)
}

func testBoardPruningDisabled(t *testing.T) {
	(*config.Get()).PruneBoards = false

	err := WriteBoard(nil, BoardConfigs{
		Created: time.Now().Add(-eightDays),
		BoardConfigs: config.BoardConfigs{
			ID:        "x",
			Eightball: []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := deleteUnusedBoards(); err != nil {
		t.Fatal(err)
	}
	assertBoardDeleted(t, "x", false)
}

func testDeleteUnusedBoards(t *testing.T) {
	(*config.Get()).PruneBoards = true
	fresh := time.Now()
	expired := fresh.Add(-eightDays)

	for _, id := range [...]string{"a", "c"} {
		err := WriteBoard(nil, BoardConfigs{
			Created: expired,
			BoardConfigs: config.BoardConfigs{
				ID:        id,
				Eightball: []string{},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	writeExpiringThreads(t, threadExpiryCases{
		{1, "a", expired},
		{3, "c", fresh},
	})

	if err := deleteUnusedBoards(); err != nil {
		t.Fatal(err)
	}

	cases := [...]struct {
		name, board string
		deleted     bool
	}{
		{"deleted", "a", true},
		{"untouched", "c", false},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assertBoardDeleted(t, c.board, c.deleted)
		})
	}
}

func writeExpiringThreads(t *testing.T, ops threadExpiryCases) {
	t.Helper()

	for _, op := range ops {
		unix := op.time.Unix()
		thread := Thread{
			ID:        op.id,
			Board:     op.board,
			ReplyTime: unix,
			BumpTime:  unix,
		}
		post := Post{
			StandalonePost: common.StandalonePost{
				Post: common.Post{
					ID: op.id,
				},
				Board: op.board,
				OP:    op.id,
			},
		}
		if err := WriteThread(nil, thread, post); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeleteOldThreads(t *testing.T) {
	assertTableClear(t, "boards")
	writeSampleBoard(t)
	config.Set(config.Configs{
		Public: config.Public{
			ThreadExpiryMin: 7,
			ThreadExpiryMax: 7,
		},
	})

	t.Run("no threads", func(t *testing.T) {
		(*config.Get()).PruneThreads = true
		if err := deleteOldThreads(); err != nil {
			t.Fatal(err)
		}
	})

	writeExpiringThreads(t, threadExpiryCases{
		{1, "a", time.Now().Add(-eightDays)},
		{2, "a", time.Now()},
	})

	t.Run("pruning disabled", func(t *testing.T) {
		(*config.Get()).PruneThreads = false
		if err := deleteOldThreads(); err != nil {
			t.Fatal(err)
		}
		assertThreadDeleted(t, 1, false)
		assertThreadDeleted(t, 2, false)
	})

	t.Run("deleted", func(t *testing.T) {
		(*config.Get()).PruneThreads = true
		if err := deleteOldThreads(); err != nil {
			t.Fatal(err)
		}
		assertThreadDeleted(t, 1, true)
		assertThreadDeleted(t, 2, false)
	})
}
