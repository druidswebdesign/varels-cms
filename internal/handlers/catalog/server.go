package catalog

import "github.com/yourname/varels_cms/internal/handlers/common"

// Server adapts the shared handler kernel to the catalog package. It embeds
// *common.Server so these handlers keep the database, session and rendering
// dependencies without importing another category package.
type Server struct{ *common.Server }

// New wraps a shared handler kernel for use by the catalog handlers.
func New(s *common.Server) *Server { return &Server{Server: s} }
