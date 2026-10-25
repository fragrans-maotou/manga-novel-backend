package domain

// MediaType matches PostgreSQL media_type_enum and Redis health key media_type.
type MediaType string

const (
	MediaTypeNovel MediaType = "novel"
	MediaTypeManga MediaType = "manga"
)

func (m MediaType) Valid() bool {
	return m == MediaTypeNovel || m == MediaTypeManga
}

// WorkStatus matches PostgreSQL work_status_enum.
type WorkStatus string

const (
	WorkStatusOngoing   WorkStatus = "ongoing"
	WorkStatusCompleted WorkStatus = "completed"
	WorkStatusHiatus    WorkStatus = "hiatus"
)

func (s WorkStatus) Valid() bool {
	switch s {
	case WorkStatusOngoing, WorkStatusCompleted, WorkStatusHiatus:
		return true
	default:
		return false
	}
}
