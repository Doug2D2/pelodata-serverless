package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Endpoint:
//   GET https://api.onepeloton.com/api/v2/ride/archived

// Query Params:
//   browse_category - looks like it matches up to fitness_discipline. Ex) cycling, yoga
//   content_format - format of context. Ex) audio, video
//   limit - number of results to return
//   page - Used for pagination, page starts at 0
//   sort_by - How to sort results.
//   	One of: original_air_time, trending, popularity, top_rated, difficulty
//   desc - Show sort descending. Should be true or false
//   is_favorite_ride - If true shows bookmarked rides. Should be true or false
//   has_workout - If true shows workouts already taken. Should be true or false
//   duration - length of class in seconds
//   class_type_id - ID of class type. Ex) Climb, Power Zone, etc.
//   instructor_id - ID of instructor.
//   super_genre_id - ID of music genre

type workout struct {
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Difficulty      float32 `json:"difficulty_estimate"`
	Duration        int     `json:"duration"`
	ImageURL        string  `json:"image_url"`
	InstructorID    string  `json:"instructor_id"`
	InstructorName  string  `json:"instructor_name"`
	OriginalAirTime int64   `json:"original_air_time"`
}

type instructor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type getWorkoutsResponse struct {
	Data           []workout    `json:"data"`
	Page           int          `json:"page"`
	TotalWorkouts  int          `json:"total"`
	WorkoutsInPage int          `json:"count"`
	NumPages       int          `json:"page_count"`
	Instructors    []instructor `json:"instructors"`
}

const basePelotonURL = "https://api.onepeloton.com"

func getWorkouts(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	url := fmt.Sprintf("%s/api/v2/ride/archived?", basePelotonURL)

	// Check for query parameters
	if cat, ok := request.QueryStringParameters["category"]; ok {
		url = fmt.Sprintf("%sbrowse_category=%s&", url, cat)
	}
	if format, ok := request.QueryStringParameters["content_format"]; ok {
		url = fmt.Sprintf("%scontent_format=%s&", url, format)
	}
	if isFavRideStr, ok := request.QueryStringParameters["is_favorite_ride"]; ok {
		isFavRide, err := strconv.ParseBool(isFavRideStr)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("is_favorite_ride parameter must be true or false")
		}
		url = fmt.Sprintf("%sis_favorite_ride=%v&", url, isFavRide)
	}
	if hasWorkoutStr, ok := request.QueryStringParameters["has_workout"]; ok {
		hasWorkout, err := strconv.ParseBool(hasWorkoutStr)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("has_workout parameter must be true or false")
		}
		url = fmt.Sprintf("%shas_workout=%v&", url, hasWorkout)
	}
	if durationStr, ok := request.QueryStringParameters["duration"]; ok {
		duration, err := strconv.Atoi(durationStr)
		if err != nil || duration < 1 {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("duration parameter must be a number greater than 0")
		}
		url = fmt.Sprintf("%sduration=%d&", url, duration)
	}
	if classType, ok := request.QueryStringParameters["class_type_id"]; ok {
		url = fmt.Sprintf("%sclass_type_id=%s&", url, classType)
	}
	if instructor, ok := request.QueryStringParameters["instructor_id"]; ok {
		url = fmt.Sprintf("%sinstructor_id=%s&", url, instructor)
	}
	if genre, ok := request.QueryStringParameters["super_genre_id"]; ok {
		url = fmt.Sprintf("%ssuper_genre_id=%s&", url, genre)
	}
	if limitStr, ok := request.QueryStringParameters["limit"]; ok {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("limit parameter must be a number greater than 0")
		}
		url = fmt.Sprintf("%slimit=%d&", url, limit)
	}
	if pageStr, ok := request.QueryStringParameters["page"]; ok {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 0 {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("page parameter must be a number 0 or greater")
		}
		url = fmt.Sprintf("%spage=%d&", url, page)
	}
	if sortBy, ok := request.QueryStringParameters["sort_by"]; ok {
		url = fmt.Sprintf("%ssort_by=%s&", url, sortBy)
	}
	if descStr, ok := request.QueryStringParameters["desc"]; ok {
		desc, err := strconv.ParseBool(descStr)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("desc parameter must be true or false")
		}
		url = fmt.Sprintf("%sdesc=%v&", url, desc)
	}

	url = strings.TrimRight(url, "&")

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to generate http request: %s", err)
	}

	// Add peloton cookie header
	if cookie, ok := request.Headers["Cookie"]; ok {
		req.Header.Add("Cookie", cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to get workouts from Peloton: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		return events.APIGatewayProxyResponse{
			StatusCode: resp.StatusCode,
		}, fmt.Errorf("Error communicating with Peloton: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to read response body: %s", err)
	}

	getWorkoutsRes := &getWorkoutsResponse{}
	err = json.Unmarshal(body, getWorkoutsRes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to unmarshal response: %s", err)
	}

	// Set instructor name for each workout
	for idx, d := range getWorkoutsRes.Data {
		for _, i := range getWorkoutsRes.Instructors {
			if d.InstructorID == i.ID {
				getWorkoutsRes.Data[idx].InstructorName = i.Name
				break
			}
		}
	}

	reply, err := json.Marshal(getWorkoutsRes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal response: %s", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode:        http.StatusOK,
		MultiValueHeaders: resp.Header,
		Body:              string(reply),
	}, nil
}

func main() {
	lambda.Start(getWorkouts)
}
