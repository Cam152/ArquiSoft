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
        _service = new AuditService(_repo, new FixedClock(new DateTimeOffset(2026, 11, 3, 15, 30, 0, TimeSpan.Zero)));
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
        Assert.Equal(new AuditEventDto("e1", "VoteRegistered", 4, "2026-11-03T14:00:00Z", "2026-11-03T15:00:00Z"), e);
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
    public async Task SummaryConIdInvalidoDa422()
    {
        var result = Assert.IsType<ObjectResult>(await _controller.Summary("x", default));
        Assert.Equal(422, result.StatusCode);
    }
}
