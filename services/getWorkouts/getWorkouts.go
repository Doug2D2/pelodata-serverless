package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Doug2D2/pelodata-serverless/services/shared"
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

type instructor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type getWorkoutsResponse struct {
	Data           []shared.Workout `json:"data"`
	Page           int              `json:"page"`
	TotalWorkouts  int              `json:"total"`
	WorkoutsInPage int              `json:"count"`
	NumPages       int              `json:"page_count"`
	Instructors    []instructor     `json:"instructors"`
}

func getQueryParams(url string, request events.APIGatewayV2HTTPRequest) (string, error) {
	if cat, ok := request.QueryStringParameters["category"]; ok {
		url = fmt.Sprintf("%sbrowse_category=%s&", url, cat)
	}
	if format, ok := request.QueryStringParameters["content_format"]; ok {
		url = fmt.Sprintf("%scontent_format=%s&", url, format)
	}
	if isFavRideStr, ok := request.QueryStringParameters["is_favorite_ride"]; ok {
		isFavRide, err := strconv.ParseBool(isFavRideStr)
		if err != nil {
			return "", errors.New("is_favorite_ride must be true or false")
		}
		url = fmt.Sprintf("%sis_favorite_ride=%v&", url, isFavRide)
	}
	if hasWorkoutStr, ok := request.QueryStringParameters["has_workout"]; ok {
		hasWorkout, err := strconv.ParseBool(hasWorkoutStr)
		if err != nil {
			return "", errors.New("has_workout must be true or false")
		}
		url = fmt.Sprintf("%shas_workout=%v&", url, hasWorkout)
	}
	if durationStr, ok := request.QueryStringParameters["duration"]; ok {
		duration, err := strconv.Atoi(durationStr)
		if err != nil || duration < 1 {
			return "", errors.New("duration must be a number greater than 0")
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
			return "", errors.New("limit must be a number greater than 0")
		}
		url = fmt.Sprintf("%slimit=%d&", url, limit)
	}
	if pageStr, ok := request.QueryStringParameters["page"]; ok {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 0 {
			return "", errors.New("page must be a number 0 or greater")
		}
		url = fmt.Sprintf("%spage=%d&", url, page)
	}
	if sortBy, ok := request.QueryStringParameters["sort_by"]; ok {
		url = fmt.Sprintf("%ssort_by=%s&", url, sortBy)
	}
	if descStr, ok := request.QueryStringParameters["desc"]; ok {
		desc, err := strconv.ParseBool(descStr)
		if err != nil {
			return "", errors.New("desc must be true or false")
		}
		url = fmt.Sprintf("%sdesc=%v&", url, desc)
	}

	return strings.TrimRight(url, "&"), nil
}

func getWorkouts(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	method := "GET"
	url := "/api/v2/ride/archived?"
	headers := map[string]string{}
	var err error

	url, err = getQueryParams(url, request)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "%s"
		}`, http.StatusBadRequest, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	// Add peloton cookie header
	if cookie, ok := request.Headers["Cookie"]; ok {
		headers["Cookie"] = cookie
	}

	body, respHeaders, resCode, err := shared.PelotonRequest(method, url, headers, nil)
	if err != nil {
		res := events.APIGatewayProxyResponse{
			StatusCode: resCode,
			Body:       err.Error(),
		}

		if body != nil {
			res.Body = string(body)
		}

		return res, nil
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
		MultiValueHeaders: respHeaders,
		Body:              string(reply),
	}, nil
}

func main() {
	lambda.Start(getWorkouts)
}
