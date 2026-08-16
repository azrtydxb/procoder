// Clean fixture — real near-misses for every rule dotnet.js implements, all
// of which must stay silent. Shares `namespace Fixtures;` with dirty.cs but
// declares a distinct class name — see the note there.
namespace Fixtures;

using System;
using System.Data.SqlClient;
using System.Security.Cryptography;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Logging;

class CleanExamples
{
    private readonly ILogger _logger;

    CleanExamples(ILogger logger)
    {
        _logger = logger;
    }

    void LookupUser(SqlCommand cmd, string id)
    {
        cmd.CommandText = "SELECT * FROM t WHERE id = @id";
        cmd.Parameters.AddWithValue("@id", id);
    }

    // FromSqlInterpolated takes a C# interpolated string by design and
    // parameterizes it internally — this is the safe EF Core API, not a
    // string-concatenation hole, even though at a glance it resembles the
    // unsafe raw-SQL call it replaces.
    void LookupUserEf(DbContext context, int id)
    {
        context.Set<object>().FromSqlInterpolated($"SELECT * FROM Users WHERE Id = {id}");
    }

    void Deserialize(string json)
    {
        System.Text.Json.JsonSerializer.Deserialize<object>(json);
    }

    void HashPassword(string password)
    {
        SHA256.Create();
    }

    string MakeToken()
    {
        var buf = RandomNumberGenerator.GetBytes(32);
        return Convert.ToBase64String(buf);
    }

    // System.Random used for a non-security purpose (display shuffling)
    // must not be confused with a security-relevant token/key/secret.
    void ShuffleDisplayOrder(int[] items)
    {
        var rng = new Random();
        Array.Sort(items, (a, b) => rng.Next(-1, 2));
    }

    void SecureHandler()
    {
        ServicePointManager.ServerCertificateValidationCallback = (a, b, c, d) => c == 0;
    }

    void Handled()
    {
        try
        {
            Go();
        }
        catch (Exception e)
        {
            _logger.LogError(e, "failed");
            throw;
        }
    }

    void DebugPrint(int x)
    {
        _logger.LogInformation("here {X}", x);
    }

    void RunGitLog(string branch)
    {
        var psi = new ProcessStartInfo
        {
            FileName = "git",
            ArgumentList = { "log", branch },
            UseShellExecute = false,
        };
        Process.Start(psi);
    }

    void Go()
    {
    }

    // Documentation that warns against a practice must not be flagged for the
    // practice: every rule dotnet.js has, named in prose, still silent.
    //
    //   never context.Users.FromSqlRaw($"SELECT * FROM Users WHERE Id = {id}")
    //   never cmd.CommandText = "SELECT * FROM t WHERE id = " + id
    //   never new BinaryFormatter() or TypeNameHandling = TypeNameHandling.All
    //   never MD5.Create() for a password, never var token = new Random().Next()
    //   never ServerCertificateValidationCallback = (a, b, c, d) => true
    //   never Process.Start($"git log {branch}")
    //   never catch (Exception) { } — log it and rethrow
    //   no leftover Console.WriteLine("here")
    void Documented()
    {
    }
}
