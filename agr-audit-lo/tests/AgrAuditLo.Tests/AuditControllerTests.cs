using System.Text;
using Agora.Audit.Controllers;
using Agora.Audit.Services;
using Microsoft.AspNetCore.Mvc;
using Xunit;

namespace Agora.Audit.Tests;

public class AuditControllerTests
{
    private readonly InMemoryAuditRepository _repo = new();
    private readonly AuditService _service;
    private readonly AuditController _controller;

    public AuditControllerTests()
    {
        _service = new AuditService(_repo, new FixedClock(new DateTimeOffset(2026, 11, 3, 15, 30, 0, TimeSpan.Zero)), TestChain.Create());
        _controller = new AuditController(_service);
    }

    private Task Seed(string id, long election) =>
        _service.RecordAsync(Encoding.UTF8.GetBytes(
            $$"""{"event_id":"{{id}}","type":"VoteRegistered","version":1,"election_id":{{election}},"occurred_at":"2026-11-03T14:00:00Z"}"""),
            default);

    [Fact]
    public async Task EventsDevuelveLaListaConFechasIso()
    {
        await Seed("e1", 4);

        var ok = Assert.IsType<OkObjectResult>(await _controller.Events("4", "5", default));
        var list = Assert.IsAssignableFrom<IEnumerable<AuditEventDto>>(ok.Value).ToList();
        var e = Assert.Single(list);
        Assert.Equal(new AuditEventDto("e1", "VoteRegistered", 4, "2026-11-03T14:00:00Z", "2026-11-03T15:00:00Z"),
            e with { Seq = null, PrevHash = null, Hash = null });
        Assert.Equal(1, e.Seq);
        Assert.Equal(new string('0', 64), e.PrevHash);
        Assert.Matches("^[0-9a-f]{64}$", e.Hash);
    }

    [Fact]
    public async Task EventsSinFiltrosUsaElLimitePorDefecto()
    {
        for (var i = 0; i < AuditService.DefaultLimit + 5; i++)
        {
            await Seed($"e{i:D3}", 1);
        }
        var ok = Assert.IsType<OkObjectResult>(await _controller.Events(null, null, default));
        Assert.Equal(AuditService.DefaultLimit, Assert.IsAssignableFrom<IEnumerable<AuditEventDto>>(ok.Value).Count());
    }

    [Theory]
    [InlineData("abc", null)]
    [InlineData("0", null)]
    [InlineData("-1", null)]
    [InlineData(null, "0")]
    [InlineData(null, "1001")]
    [InlineData(null, "diez")]
    public async Task EventsConParametrosInvalidosDa422(string? electionId, string? limit)
    {
        var result = Assert.IsType<ObjectResult>(await _controller.Events(electionId, limit, default));
        Assert.Equal(422, result.StatusCode);
        Assert.IsType<ErrorDto>(result.Value);
    }

    [Fact]
    public async Task SummaryCuentaLosEventosDeLaEleccion()
    {
        await Seed("a", 2);
        await Seed("b", 2);
        await Seed("c", 3);

        var ok = Assert.IsType<OkObjectResult>(await _controller.Summary("2", default));
        Assert.Equal(new SummaryDto(2, 2), ok.Value);
    }

    [Fact]
    public async Task VerifyDevuelveElInformeDeLaCadena()
    {
        await Seed("a", 1);
        await Seed("b", 1);
        _repo.Events[0].ElectionId = 9;

        var ok = Assert.IsType<OkObjectResult>(await _controller.Verify(default));
        var report = Assert.IsType<VerifyReportDto>(ok.Value);
        Assert.False(report.Valid);
        Assert.Equal(2, report.TotalEvents);
        Assert.Equal(2, report.HeadSeq);
        Assert.Equal("2026-11-03T15:30:00Z", report.CheckedAt);
        var problem = Assert.Single(report.Problems);
        Assert.Equal("altered", problem.Kind);
        Assert.Equal(1, problem.Seq);
        Assert.Equal("a", problem.EventId);
    }

    [Fact]
    public async Task SummaryConIdInvalidoDa422()
    {
        var result = Assert.IsType<ObjectResult>(await _controller.Summary("x", default));
        Assert.Equal(422, result.StatusCode);
    }
}
