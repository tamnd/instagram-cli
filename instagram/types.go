package instagram

import "time"

// Profile is a public Instagram user profile.
type Profile struct {
	Username   string    `json:"username"     kit:"id"`
	FullName   string    `json:"full_name,omitempty"`
	Bio        string    `json:"bio,omitempty"        table:",truncate"`
	Website    string    `json:"website,omitempty"    table:",truncate"`
	Followers  int64     `json:"followers,omitempty"`
	Following  int64     `json:"following,omitempty"`
	Posts      int64     `json:"posts,omitempty"`
	IsPrivate  bool      `json:"is_private,omitempty"`
	IsVerified bool      `json:"is_verified,omitempty"`
	ProfilePic string    `json:"profile_pic_url,omitempty" table:",truncate"`
	URL        string    `json:"url"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// Post is one item in a user's public timeline.
type Post struct {
	ID        string    `json:"id"              kit:"id"`
	ShortCode string    `json:"shortcode,omitempty"`
	URL       string    `json:"url"`
	Type      string    `json:"type,omitempty"`
	Caption   string    `json:"caption,omitempty"   table:",truncate"`
	Thumbnail string    `json:"thumbnail,omitempty" table:",truncate"`
	Likes     int64     `json:"likes,omitempty"`
	Comments  int64     `json:"comments,omitempty"`
	Timestamp string    `json:"timestamp,omitempty"`
	IsVideo   bool      `json:"is_video,omitempty"`
	VideoURL  string    `json:"video_url,omitempty" table:",truncate"`
	FetchedAt time.Time `json:"fetched_at"`
}

// --- wire types (Instagram API JSON) ---

// rawResponse is the top-level shape returned by /api/v1/users/web_profile_info/.
type rawResponse struct {
	Data   rawData `json:"data"`
	Status string  `json:"status"`
	// Blocked response carries "message" at top level.
	Message string `json:"message"`
}

type rawData struct {
	User *rawUser `json:"user"`
}

type rawUser struct {
	ID         string   `json:"id"`
	Username   string   `json:"username"`
	FullName   string   `json:"full_name"`
	Biography  string   `json:"biography"`
	ExternalURL string  `json:"external_url"`
	IsPrivate  bool     `json:"is_private"`
	IsVerified bool     `json:"is_verified"`
	ProfilePicURL string `json:"profile_pic_url"`
	EdgeFollowedBy rawCount       `json:"edge_followed_by"`
	EdgeFollow     rawCount       `json:"edge_follow"`
	EdgeMedia      rawEdgeMedia   `json:"edge_owner_to_timeline_media"`
}

type rawCount struct {
	Count int64 `json:"count"`
}

type rawEdgeMedia struct {
	Count int64          `json:"count"`
	Edges []rawEdgeNode  `json:"edges"`
}

type rawEdgeNode struct {
	Node rawMediaNode `json:"node"`
}

type rawMediaNode struct {
	ID          string       `json:"id"`
	ShortCode   string       `json:"shortcode"`
	TypeName    string       `json:"__typename"`
	ThumbnailSrc string     `json:"thumbnail_src"`
	DisplayURL  string       `json:"display_url"`
	IsVideo     bool         `json:"is_video"`
	VideoURL    string       `json:"video_url"`
	TakenAt     int64        `json:"taken_at_timestamp"`
	Caption     rawCapEdge   `json:"edge_media_to_caption"`
	LikeCount   rawCount     `json:"edge_media_preview_like"`
	CommentCount rawCount    `json:"edge_media_to_comment"`
}

type rawCapEdge struct {
	Edges []rawCapNode `json:"edges"`
}

type rawCapNode struct {
	Node struct {
		Text string `json:"text"`
	} `json:"node"`
}
