using System.Text;
using Agora.Audit.Models;
using Xunit;

namespace Agora.Audit.Tests;

public class VoteRegisteredTests
{
    // Mensaje tal como lo publica agr-vote-lo (README, sección 5.3).
    internal const string Valid =
        """{"event_id":"6720f3a1c2b9e8a4d1f00a17","type":"VoteRegistered","version":1,"election_id":1,"candidate_id":2,"occurred_at":"2026-11-03T14:00:00Z"}""";

    private static bool Parse(string json, out VoteRegistered? evt, out string error) =>
        VoteRegistered.TryParse(Encoding.UTF8.GetBytes(json), out evt, out error);

    [Fact]
    public void LeeElContratoV1()
    {
        Assert.True(Parse(Valid, out var evt, out _));
        Assert.Equal("6720f3a1c2b9e8a4d1f00a17", evt!.EventId);
        Assert.Equal("VoteRegistered", evt.Type);
        Assert.Equal(1, evt.Version);
        Assert.Equal(1, evt.ElectionId);
        Assert.Equal(new DateTimeOffset(2026, 11, 3, 14, 0, 0, TimeSpan.Zero), evt.OccurredAt);
    }

    [Fact]
    public void AceptaCamposNuevosYSinCandidato()
    {
        Assert.True(Parse(
            """{"event_id":"a","type":"VoteRegistered","version":1,"election_id":3,"occurred_at":"2026-11-03T14:00:00Z","otro":true}""",
            out var evt, out _));
        Assert.Equal(3, evt!.ElectionId);
    }

    [Theory]
    [InlineData("no es json")]
    [InlineData("[]")]
    [InlineData("""{"type":"VoteRegistered","version":1,"election_id":1,"occurred_at":"2026-11-03T14:00:00Z"}""")]
    [InlineData("""{"event_id":"","type":"VoteRegistered","version":1,"election_id":1,"occurred_at":"2026-11-03T14:00:00Z"}""")]
    [InlineData("""{"event_id":"a","type":"OtroEvento","version":1,"election_id":1,"occurred_at":"2026-11-03T14:00:00Z"}""")]
    [InlineData("""{"event_id":"a","type":"VoteRegistered","version":2,"election_id":1,"occurred_at":"2026-11-03T14:00:00Z"}""")]
    [InlineData("""{"event_id":"a","type":"VoteRegistered","version":1,"election_id":0,"occurred_at":"2026-11-03T14:00:00Z"}""")]
    [InlineData("""{"event_id":"a","type":"VoteRegistered","version":1,"election_id":"1","occurred_at":"2026-11-03T14:00:00Z"}""")]
    [InlineData("""{"event_id":"a","type":"VoteRegistered","version":1,"election_id":1,"occurred_at":"ayer"}""")]
    [InlineData("""{"event_id":"a","type":"VoteRegistered","version":1,"election_id":1}""")]
    public void RechazaMensajesQueNoCumplenElContrato(string json)
    {
        Assert.False(Parse(json, out var evt, out var error));
        Assert.Null(evt);
        Assert.NotEmpty(error);
    }
}
