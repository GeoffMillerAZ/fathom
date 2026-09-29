module example.com/broken

go 1.20

require (
	github.com/ok/one v1.0.0
	this line is not a valid requirement at all !!!
	github.com/ok/two v1.2.0
)

bogus directive here

require github.com/ok/three v0.3.0
