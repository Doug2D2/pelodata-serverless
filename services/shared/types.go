package shared

type Workout struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Difficulty      float32 `json:"difficulty_estimate"`
	Duration        int     `json:"duration"`
	ImageURL        string  `json:"image_url"`
	InstructorID    string  `json:"instructor_id"`
	InstructorName  string  `json:"instructor_name"`
	OriginalAirTime int64   `json:"original_air_time"`
}
