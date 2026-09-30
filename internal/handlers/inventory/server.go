package inventory

import "github.com/druidswebdesign/varels-cms/internal/handlers/common"

// Server adapts the shared handler kernel to the inventory package. It embeds
// *common.Server so these handlers keep the database, session and rendering
// dependencies without importing another category package.
type Server struct{ *common.Server }

// New wraps a shared handler kernel for use by the inventory handlers.
func New(s *common.Server) *Server { return &Server{Server: s} }
