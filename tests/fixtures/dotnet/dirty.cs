// Deliberately unsafe/broken fixture — exercises every dotnet.js finding id.
//
// Both files here declare `namespace Fixtures;` (the file-scoped form) — a
// real C# project would collide if two files in one directory declared the
// same namespace *and* the same class name, so DirtyExamples and
// CleanExamples (in clean.cs) are kept distinct even though the namespace is
// shared. File-scoped namespace syntax also avoids an extra brace level of
// pure structural nesting that would otherwise trip the shared
// obvious/nesting-depth check on scoping alone, before any real logic.
namespace Fixtures;

using System;
using System.Data.SqlClient;
using System.Runtime.Serialization.Formatters.Binary;
using System.Security.Cryptography;

class DirtyExamples
{
    void LookupUser(SqlConnection conn, string id)
    {
        var cmd = new SqlCommand($"SELECT * FROM t WHERE id = {id}", conn);
        cmd.CommandText = "SELECT * FROM t WHERE id = " + id;
    }

    void Deserialize(System.IO.Stream payload)
    {
        var f = new BinaryFormatter();
        f.Deserialize(payload);
    }

    void HashPassword(string password)
    {
        MD5.Create();
    }

    string MakeToken()
    {
        var token = new Random().Next().ToString();
        return token;
    }

    void InsecureHandler()
    {
        ServicePointManager.ServerCertificateValidationCallback = (a, b, c, d) => true;
    }

    void Swallow()
    {
        try
        {
            Go();
        }
        catch (Exception)
        {
        }
    }

    void DebugPrint(int x)
    {
        Console.WriteLine("here " + x);
    }

    void RunGitLog(string branch)
    {
        Process.Start($"git log {branch}");
    }

    void RunShell(string cmd)
    {
        var psi = new ProcessStartInfo { FileName = "cmd.exe", Arguments = $"/c {cmd}", UseShellExecute = true };
        Process.Start(psi);
    }

    void Go()
    {
    }
}
