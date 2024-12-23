package server

// APIResponse represents the structure of a standard API response.
// It contains a success flag, a message, and optional data.
// Fields:
// - Success: Indicates whether the API request was successful.
// - Message: Provides additional information about the API response.
// - Data: Contains the response data, if any. This field is optional and will be omitted if empty.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// FileUploadData represents the data associated with a file upload.
// It contains the filename and the size of the uploaded file.
type FileUploadData struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}
