using Microsoft.AspNetCore.Mvc;

namespace Agora.Audit.Controllers;

[ApiController]
public sealed class HealthController : ControllerBase
{
    /// <summary>GET /health: lo usa el healthcheck de Docker Compose.</summary>
    [HttpGet("/health")]
    public IActionResult Health() => Ok(new { status = "ok" });
}
