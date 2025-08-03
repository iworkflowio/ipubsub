package databases

type (
	MessageStore interface {
		Close() error
	}
)
