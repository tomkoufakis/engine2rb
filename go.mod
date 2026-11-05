module engine2rb

go 1.24.2

require (
	fyne.io/fyne/v2 v2.8.2
	github.com/mattn/go-sqlite3 v1.14.28
	github.com/nateranda/djtools v0.1.0
)

// djtools is used from the fork in third_party/djtools, which adds snapshot
// import, playlist filtering and several fixes. Point this at your own fork's
// module path/version instead once it is published.
replace github.com/nateranda/djtools => ./third_party/djtools
