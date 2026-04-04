package test

import (
	"github.com/draftloop/elm"
	"log"
	_ "modernc.org/sqlite"
	"reflect"
	"testing"
	"time"
)

func ptr[T any](v T) *T {
	return &v
}

func Test(t *testing.T) {
	db, err := elm.Open("sqlite", ":memory:", elm.Config{
		Logger: func(query string, args []any, duration time.Duration, err error) {
			if err != nil {
				log.Printf("ERR %s %v (%s): %v", query, args, duration, err)
			} else {
				log.Printf("%s %v (%s)", query, args, duration)
			}
		},
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	type Foo struct{ ID int64 }
	type Bar struct{ ID int64 }

	var foos []Foo
	if err = db.Model(Foo{}).LeftRelation(Bar{}).Scan(&foos); err == nil {
		t.Fatal("expected error for undeclared LeftRelation, got nil")
	}
	if err = db.Model(Foo{}).SelectFrom(Foo{}, "NonExistent").Scan(&foos); err == nil {
		t.Fatal("expected error for unknown SelectFrom field, got nil")
	}

	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL, fake_foreign_key_id INTEGER, always_null_text TEXT)")
	if err != nil {
		t.Fatalf("create table users: %v", err)
	}

	type User struct {
		ID               int64
		Username         string
		FakeForeignKeyID *int64
	}

	user1 := &User{Username: "foo"}
	user2 := User{Username: "bar", FakeForeignKeyID: ptr(int64(10001))}
	var user3 *User
	user3 = &User{Username: "baz"}

	if err = db.Save(user1); err != nil {
		t.Fatalf("Save(*User): %v", err)
	}
	if user1.ID != int64(1) {
		t.Fatalf("expected user1.ID = 1, got %d", user1.ID)
	}

	if err = db.Save(user2); err == nil {
		t.Fatal("Save(User) should return error for non-pointer, got nil")
	}
	if err = db.Save(&user2); err != nil {
		t.Fatalf("Save(&User): %v", err)
	}
	if user2.ID != int64(2) {
		t.Fatalf("expected user2.ID = 2, got %d", user2.ID)
	}

	if err = db.Save(user3); err != nil {
		t.Fatalf("Save(*User) insert: %v", err)
	}
	user3.Username = "qux"
	if err = db.Save(user3); err != nil {
		t.Fatalf("Save(*User) update: %v", err)
	}

	user3 = nil
	if err = db.Model(User{}).
		UnsafeOrderBy("id desc").
		Scan(&user3); err != nil {
		t.Fatalf("Scan(&*User) last: %v", err)
	}
	if user3.Username != "qux" {
		t.Fatalf("expected user3.Username = qux, got %q", user3.Username)
	}

	var lastUsername string
	if err = db.Model(User{}).
		UnsafeSelect("username").
		UnsafeOrderBy("id desc").
		Scan(&lastUsername); err != nil {
		t.Fatalf("Scan(&string) last username: %v", err)
	}
	if lastUsername != "qux" {
		t.Fatalf("expected lastUsername = qux, got %q", lastUsername)
	}

	var alwaysNullTextField string
	if err = db.Model(User{}).
		UnsafeSelect("always_null_text").
		Scan(&alwaysNullTextField); err != nil {
		t.Fatalf("Scan(&string) always_null_text: %v", err)
	}
	if alwaysNullTextField != "" {
		t.Fatalf("expected alwaysNullTextField = '', got %q", alwaysNullTextField)
	}

	var alwaysNullTextFieldNullable *string
	if err = db.Model(User{}).
		UnsafeSelect("always_null_text").
		Scan(&alwaysNullTextFieldNullable); err != nil {
		t.Fatalf("Scan(&*string) always_null_text: %v", err)
	}
	if alwaysNullTextFieldNullable != nil {
		t.Fatalf("expected alwaysNullTextFieldNullable = nil, got %q", *alwaysNullTextFieldNullable)
	}

	var usernames []string
	if err = db.Model(User{}).
		UnsafeSelect("username").
		UnsafeOrderBy("username asc").
		Scan(&usernames); err != nil {
		t.Fatalf("Scan([]string) usernames: %v", err)
	}
	if !reflect.DeepEqual(usernames, []string{"bar", "foo", "qux"}) {
		t.Fatalf("expected %v, got %v", []string{"bar", "foo", "qux"}, usernames)
	}

	var unknownUser User
	if err = db.Model(User{}).
		Where(elm.Eq("id", int64(10))).
		Scan(&unknownUser); err != nil {
		t.Fatalf("Scan(&User) unknown: %v", err)
	}
	if unknownUser.ID != 0 {
		t.Fatalf("expected unknownUser.ID = 0, got %d", unknownUser.ID)
	}

	var unknownUserNullable *User
	if err = db.Model(User{}).
		Where(elm.Eq("id", int64(10))).
		Scan(&unknownUserNullable); err != nil {
		t.Fatalf("Scan(&*User) unknown: %v", err)
	}
	if unknownUserNullable != nil {
		t.Fatalf("expected unknownUserNullable = nil, got id %d", unknownUserNullable.ID)
	}

	var users []User
	if err = db.Model(User{}).
		UnsafeOrderBy("id asc").
		Scan(&users); err != nil {
		t.Fatalf("Scan([]User): %v", err)
	}
	usersExpected := []User{
		{ID: 1, Username: "foo", FakeForeignKeyID: nil},
		{ID: 2, Username: "bar", FakeForeignKeyID: ptr(int64(10001))},
		{ID: 3, Username: "qux", FakeForeignKeyID: nil},
	}
	if !reflect.DeepEqual(users, usersExpected) {
		t.Fatalf("users mismatch\n users: %#v\nexpected: %#v", users, usersExpected)
	}

	var usersNullable []*User
	if err = db.Model(User{}).
		UnsafeOrderBy("id asc").
		Scan(&usersNullable); err == nil {
		t.Fatal("Scan([]*User) should return error, got nil")
	}

	if err = db.Delete(&user3); err != nil {
		t.Fatalf("Delete(*User): %v", err)
	}
	if user3 != nil {
		t.Fatalf("Delete should zero out *User, got id=%d", user3.ID)
	}

	_, err = db.Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY, message TEXT NOT NULL, user_id INTEGER, FOREIGN KEY(user_id) REFERENCES users(id))")
	if err != nil {
		t.Fatalf("create table posts: %v", err)
	}

	type Post struct {
		ID      int64
		Message string
		UserID  *int64

		User *User
	}

	post1 := Post{UserID: ptr(user1.ID), Message: "msg 1 from user1"}
	if err = db.Save(&post1); err != nil {
		t.Fatalf("Save(&Post): %v", err)
	}

	post2 := Post{UserID: ptr(user1.ID), Message: "msg 2 from user1"}
	if err = db.Save(&post2); err != nil {
		t.Fatalf("Save(&Post): %v", err)
	}

	post3 := Post{UserID: ptr(user2.ID), Message: "msg 3 from user2"}
	if err = db.Save(&post3); err != nil {
		t.Fatalf("Save(&Post): %v", err)
	}

	post4 := Post{UserID: nil, Message: "msg 3 from deleted user"}
	if err = db.Save(&post4); err != nil {
		t.Fatalf("Save(&Post): %v", err)
	}

	var postsWithUser []Post
	if err = db.Model(Post{}).LeftRelation(User{}).UnsafeOrderBy("Post.id asc").Scan(&postsWithUser); err != nil {
		t.Fatalf("Scan(&[]Post): %v", err)
	}
	if len(postsWithUser) != 4 {
		t.Fatalf("expected len(postsWithUser) = 4, got %d", len(postsWithUser))
	} else if postsWithUser[0].ID != 1 {
		t.Fatalf("expected postsWithUser[0].ID = 1, got %d", postsWithUser[0].ID)
	} else if postsWithUser[1].ID != 2 {
		t.Fatalf("expected postsWithUser[1].ID = 2, got %d", postsWithUser[1].ID)
	} else if postsWithUser[2].ID != 3 {
		t.Fatalf("expected postsWithUser[2].ID = 3, got %d", postsWithUser[2].ID)
	} else if postsWithUser[3].ID != 4 {
		t.Fatalf("expected postsWithUser[3].ID = 4, got %d", postsWithUser[3].ID)
	}
	if !(postsWithUser[0].UserID != nil && *postsWithUser[0].UserID == user1.ID) {
		t.Fatalf("expected postsWithUser[0].UserID = %d, got %d", user1.ID, postsWithUser[0].UserID)
	} else if !(postsWithUser[0].User != nil && postsWithUser[0].User.ID == user1.ID) {
		t.Fatalf("expected postsWithUser[0].User.ID = %d, got %d", user1.ID, postsWithUser[0].User.ID)
	}
	if !(postsWithUser[1].UserID != nil && *postsWithUser[1].UserID == user1.ID) {
		t.Fatalf("expected postsWithUser[1].UserID = %d, got %d", user1.ID, postsWithUser[1].UserID)
	} else if !(postsWithUser[1].User != nil && postsWithUser[1].User.ID == user1.ID) {
		t.Fatalf("expected postsWithUser[1].User.ID = %d, got %d", user1.ID, postsWithUser[1].User.ID)
	}
	if !(postsWithUser[2].UserID != nil && *postsWithUser[2].UserID == user2.ID) {
		t.Fatalf("expected postsWithUser[2].UserID = %d, got %d", user2.ID, postsWithUser[2].UserID)
	} else if !(postsWithUser[2].User != nil && postsWithUser[2].User.ID == user2.ID) {
		t.Fatalf("expected postsWithUser[2].User.ID = %d, got %d", user2.ID, postsWithUser[2].User.ID)
	}
	if postsWithUser[3].UserID != nil {
		t.Fatalf("expected postsWithUser[3].UserID = nil, got %d", postsWithUser[3].UserID)
	} else if postsWithUser[3].User != nil {
		t.Fatalf("expected postsWithUser[3].User = nil, got %d", postsWithUser[3].User.ID)
	}

	_, err = db.Exec("CREATE TABLE excavators (id INTEGER PRIMARY KEY, registration_number TEXT NOT NULL, purchase_date DATETIME NOT NULL, sale_date DATETIME)")
	if err != nil {
		t.Fatalf("create table excavators: %v", err)
	}

	type Excavator struct {
		ID                 int64
		RegistrationNumber string
		PurchaseDate       time.Time
		SaleDate           *time.Time
	}

	excavator1Insert, err := db.Exec("INSERT INTO excavators (registration_number, purchase_date) VALUES (?, ?)", "DGTIME", time.Now())
	if err != nil {
		t.Fatalf("insert excavator 1: %v", err)
	}
	excavator1ID, err := excavator1Insert.LastInsertId()
	if err != nil {
		t.Fatalf("excavator 1 LastInsertId: %v", err)
	}

	excavator2Insert, err := db.Exec("INSERT INTO excavators (registration_number, purchase_date, sale_date) VALUES (?, ?, ?)", "MUDHUG", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert excavator 2: %v", err)
	}
	excavator2ID, err := excavator2Insert.LastInsertId()
	if err != nil {
		t.Fatalf("excavator 2 LastInsertId: %v", err)
	}

	_, err = db.Exec("CREATE TABLE work_hours (id INTEGER PRIMARY KEY, excavator_id INTEGER NOT NULL, user_id INTEGER NOT NULL, hours INTEGER NOT NULL, FOREIGN KEY(excavator_id) REFERENCES excavators(id), FOREIGN KEY(user_id) REFERENCES users(id))")
	if err != nil {
		t.Fatalf("create table work_hours: %v", err)
	}

	type WorkHour struct {
		ID          int64
		ExcavatorID int64
		UserID      int64
		Hours       int64
	}

	for i, wh := range []struct {
		excavatorID int64
		userID      int64
		hours       int64
	}{
		{excavator1ID, user1.ID, 8},
		{excavator1ID, user1.ID, 6},
		{excavator1ID, user2.ID, 10},
		{excavator2ID, user2.ID, 4},
		{excavator2ID, user2.ID, 7},
	} {
		if _, err = db.Exec("INSERT INTO work_hours (excavator_id, user_id, hours) VALUES (?, ?, ?)", wh.excavatorID, wh.userID, wh.hours); err != nil {
			t.Fatalf("insert work_hours %d: %v", i+1, err)
		}
	}

	type UserWorkPerExcavator struct {
		Excavator Excavator
		User      *User
		Hours     int64
	}
	var usersWorkPerExcavator []UserWorkPerExcavator
	if err := db.Model(WorkHour{}).
		SelectFrom(Excavator{}, "ID", "RegistrationNumber").
		SelectAllFrom(User{}).
		UnsafeSelect("SUM(WorkHour.hours) AS hours").
		UnsafeJoin("INNER", User{}, "User.id = WorkHour.user_id").
		UnsafeJoin("INNER", Excavator{}, "Excavator.id = WorkHour.excavator_id").
		UnsafeGroupBy("Excavator.id", "User.id").
		UnsafeOrderBy("Excavator.id asc", "User.id asc").
		Scan(&usersWorkPerExcavator); err != nil {
		t.Fatalf("Scan(&[]UserWorkPerExcavator): %v", err)
	}
	if len(usersWorkPerExcavator) != 3 {
		t.Fatalf("expected len(usersWorkPerExcavator) = 3, got %d", len(usersWorkPerExcavator))
	}
	if !(usersWorkPerExcavator[0].Excavator.ID == excavator1ID && usersWorkPerExcavator[0].Excavator.RegistrationNumber == "DGTIME" && usersWorkPerExcavator[0].User != nil && usersWorkPerExcavator[0].User.Username == "foo" && usersWorkPerExcavator[0].Hours == 14) {
		t.Fatalf("unexpected usersWorkPerExcavator[0]: %#v", usersWorkPerExcavator[0])
	}
	if !(usersWorkPerExcavator[1].Excavator.ID == excavator1ID && usersWorkPerExcavator[1].Excavator.RegistrationNumber == "DGTIME" && usersWorkPerExcavator[1].User != nil && usersWorkPerExcavator[1].User.Username == "bar" && usersWorkPerExcavator[1].Hours == 10) {
		t.Fatalf("unexpected usersWorkPerExcavator[1]: %#v", usersWorkPerExcavator[1])
	}
	if !(usersWorkPerExcavator[2].Excavator.ID == excavator2ID && usersWorkPerExcavator[2].Excavator.RegistrationNumber == "MUDHUG" && usersWorkPerExcavator[2].User != nil && usersWorkPerExcavator[2].User.Username == "bar" && usersWorkPerExcavator[2].Hours == 11) {
		t.Fatalf("unexpected usersWorkPerExcavator[2]: %#v", usersWorkPerExcavator[2])
	}
}
