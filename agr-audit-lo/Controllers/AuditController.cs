using System.Globalization;
using Agora.Audit.Models;
using Agora.Audit.Services;
using Microsoft.AspNetCore.Mvc;

namespace Agora.Audit.Controllers;

/// <summary>
/// Consulta del registro (solo lectura). La llama agr-gateway-or en GET /admin/audit
/// y GET /admin/audit/verify después de verificar la llave de administración.
/// </summary>
[ApiController]
[Route("audit")]
public sealed class AuditController : ControllerBase
{
    private readonly AuditService _audit;

    public AuditController(AuditService audit)
    {
        _audit = audit;
    }

    /// <summary>GET /audit/events?election_id=1&amp;limit=50: eventos más recientes primero.</summary>
    [HttpGet("events")]
    public async Task<IActionResult> Events(
        [FromQuery(Name = "election_id")] string? electionId,
        [FromQuery(Name = "limit")] string? limit,
        CancellationToken ct)
    {
        long? election = null;
        if (!string.IsNullOrEmpty(electionId))
        {
            if (!TryPositive(electionId, out var id))
            {
                return Detail(422, "election_id debe ser un entero positivo");
            }
            election = id;
        }

        var max = AuditService.DefaultLimit;
        if (!string.IsNullOrEmpty(limit))
        {
            if (!TryPositive(limit, out var l) || l > AuditService.MaxLimit)
            {
                return Detail(422, $"limit debe estar entre 1 y {AuditService.MaxLimit}");
            }
            max = (int)l;
        }

        var events = await _audit.ListAsync(election, max, ct);
        return Ok(events.Select(AuditEventDto.From));
    }

    /// <summary>GET /audit/summary/{election_id}: total de eventos de la elección.</summary>
    [HttpGet("summary/{electionId}")]
    public async Task<IActionResult> Summary(string electionId, CancellationToken ct)
    {
        if (!TryPositive(electionId, out var id))
        {
            return Detail(422, "election_id debe ser un entero positivo");
        }
        var count = await _audit.CountAsync(id, ct);
        return Ok(new SummaryDto(id, count));
    }

    /// <summary>
    /// GET /audit/verify: recorre la cadena de hashes (S5) y dice si alguien cambió,
    /// borró o insertó eventos directamente en agr-audit-db, y cuáles.
    /// </summary>
    [HttpGet("verify")]
    public async Task<IActionResult> Verify(CancellationToken ct) =>
        Ok(VerifyReportDto.From(await _audit.VerifyAsync(ct)));

    private static bool TryPositive(string raw, out long value) =>
        long.TryParse(raw, NumberStyles.None, CultureInfo.InvariantCulture, out value) && value > 0;

    private ObjectResult Detail(int status, string detail) =>
        StatusCode(status, new ErrorDto(detail));
}

public sealed record AuditEventDto(
    string EventId, string Type, long ElectionId, string OccurredAt, string RecordedAt,
    long? Seq = null, string? PrevHash = null, string? Hash = null)
{
    public static AuditEventDto From(AuditEvent e) =>
        new(e.EventId, e.Type, e.ElectionId, Iso(e.OccurredAt), Iso(e.RecordedAt), e.Seq, e.PrevHash, e.Hash);

    internal static string Iso(DateTime value) =>
        DateTime.SpecifyKind(value, DateTimeKind.Utc).ToString("yyyy-MM-dd'T'HH:mm:ss'Z'", CultureInfo.InvariantCulture);
}

public sealed record VerifyReportDto(
    bool Valid,
    long TotalEvents,
    long ChainedEvents,
    long HeadSeq,
    string? HeadHash,
    long? KnownHeadSeq,
    string CheckedAt,
    long ProblemCount,
    IReadOnlyList<ChainProblemDto> Problems)
{
    public static VerifyReportDto From(VerifyReport r) => new(
        r.Valid, r.TotalEvents, r.ChainedEvents, r.HeadSeq, r.HeadHash, r.KnownHeadSeq,
        AuditEventDto.Iso(r.CheckedAt), r.ProblemCount,
        r.Problems.Select(p => new ChainProblemDto(p.Kind, p.Seq, p.EventId, p.Detail)).ToList());
}

public sealed record ChainProblemDto(string Kind, long? Seq, string? EventId, string Detail);

public sealed record SummaryDto(long ElectionId, long Events);

public sealed record ErrorDto(string Detail);
