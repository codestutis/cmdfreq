package report

import "html/template"

var htmlReportTemplate = template.Must(template.New("cmdfreq-report").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>cmdfreq — Your command line, ranked</title>
  <style>
    :root {
      color-scheme: dark;
      --ink: #f4f6fb;
      --muted: #9aa6bf;
      --panel: #111b2a;
      --panel-edge: #35445b;
      --page: #050b14;
      --periwinkle: #8ea2ff;
      --gold: #f5bd3e;
      --silver: #b8c1d1;
      --bronze: #cf7739;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-width: 320px;
      background: #02060d;
      color: var(--ink);
    }
    .report {
      width: min(100%, 1080px);
      min-height: 1350px;
      margin: 0 auto;
      padding: 56px 52px 42px;
      overflow: hidden;
      background:
        radial-gradient(circle at 50% 25%, rgba(70, 95, 154, .17), transparent 31%),
        linear-gradient(145deg, #091220, var(--page) 68%);
    }
    .wordmark {
      color: var(--periwinkle);
      font: 700 22px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      letter-spacing: .24em;
      text-align: center;
    }
    h1 {
      margin: 25px 0 10px;
      font-size: clamp(40px, 6vw, 64px);
      line-height: 1.02;
      letter-spacing: -.045em;
      text-align: center;
    }
    .subtitle {
      margin: 0;
      color: var(--muted);
      font-size: 20px;
      text-align: center;
    }
    .stats {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 16px;
      max-width: 760px;
      margin: 34px auto 36px;
    }
    .stat {
      padding: 23px 10px 20px;
      border: 1px solid var(--panel-edge);
      border-radius: 17px;
      background: linear-gradient(145deg, rgba(22, 35, 53, .94), rgba(11, 20, 33, .94));
      text-align: center;
      box-shadow: inset 0 1px rgba(255, 255, 255, .03);
    }
    .stat strong {
      display: block;
      font: 650 37px/1.1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    .stat span {
      display: block;
      margin-top: 10px;
      color: var(--muted);
      font: 700 13px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      letter-spacing: .08em;
      text-transform: uppercase;
    }
    .section-title {
      display: grid;
      grid-template-columns: 1fr auto 1fr;
      align-items: center;
      gap: 20px;
      margin: 0 8px 24px;
      color: var(--periwinkle);
      font: 750 20px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      letter-spacing: .14em;
      text-align: center;
      text-transform: uppercase;
    }
    .section-title::before,
    .section-title::after {
      height: 1px;
      background: var(--panel-edge);
      content: "";
    }
    .podium {
      display: grid;
      grid-template-columns: 1fr 1.14fr 1fr;
      align-items: end;
      gap: 20px;
      margin-bottom: 36px;
    }
    .podium-card {
      --medal: var(--silver);
      position: relative;
      min-height: 290px;
      padding: 24px 18px 26px;
      border: 1px solid var(--medal);
      border-radius: 18px;
      background: linear-gradient(160deg, rgba(24, 35, 50, .97), rgba(8, 16, 27, .98));
      text-align: center;
      box-shadow: 0 12px 26px rgba(0, 0, 0, .22), inset 0 -8px color-mix(in srgb, var(--medal) 68%, transparent);
    }
    .podium-card.gold { --medal: var(--gold); min-height: 335px; }
    .podium-card.bronze { --medal: var(--bronze); min-height: 270px; }
    .medal {
      display: grid;
      width: 70px;
      height: 70px;
      margin: 0 auto 18px;
      place-items: center;
      border: 4px double color-mix(in srgb, var(--medal) 74%, #000);
      border-radius: 50%;
      background: radial-gradient(circle at 35% 30%, #fff7, transparent 27%), var(--medal);
      color: #07101c;
      font: 800 34px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      box-shadow: 0 4px 14px color-mix(in srgb, var(--medal) 32%, transparent);
    }
    .podium-card h2 {
      margin: 0;
      overflow-wrap: anywhere;
      font: 750 35px/1.15 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    .podium-count {
      margin-top: 13px;
      color: var(--medal);
      font: 700 42px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    .podium-count small {
      display: block;
      margin-top: 8px;
      font-size: 13px;
      letter-spacing: .12em;
      text-transform: uppercase;
    }
    .podium-percent {
      margin-top: 20px;
      padding-top: 17px;
      border-top: 1px solid color-mix(in srgb, var(--medal) 70%, transparent);
      color: var(--medal);
      font: 700 23px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    .ranking {
      padding: 12px 24px;
      border: 1px solid var(--panel-edge);
      border-radius: 18px;
      background: rgba(14, 25, 40, .88);
      box-shadow: inset 0 1px rgba(255, 255, 255, .03);
    }
    .rank-row {
      display: grid;
      grid-template-columns: 50px minmax(100px, .72fr) 2fr 60px;
      align-items: center;
      gap: 15px;
      min-height: 58px;
      border-bottom: 1px solid rgba(87, 103, 130, .35);
      font: 650 20px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    .rank-row:last-child { border-bottom: 0; }
    .rank-number { color: var(--muted); }
    .command-name { overflow-wrap: anywhere; }
    .bar-track {
      height: 14px;
      overflow: hidden;
      border-radius: 4px;
      background: #1d2a3e;
    }
    .bar {
      display: block;
      height: 100%;
      border-radius: inherit;
      background: linear-gradient(90deg, #758ce8, #9eb0ff);
      box-shadow: 0 0 16px rgba(117, 140, 232, .22);
    }
    .rank-count { text-align: right; }
    footer {
      margin-top: 30px;
      color: var(--muted);
      font: 500 14px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      text-align: center;
    }
    footer strong {
      display: block;
      margin-top: 13px;
      color: var(--periwinkle);
      font-size: 18px;
      letter-spacing: .2em;
    }
    @media (max-width: 700px) {
      .report { min-height: 0; padding: 36px 20px 30px; }
      .stats { grid-template-columns: 1fr; max-width: 260px; }
      .podium { gap: 8px; }
      .podium-card { min-height: 250px; padding-inline: 8px; }
      .podium-card.gold { min-height: 280px; }
      .podium-card.bronze { min-height: 235px; }
      .podium-card h2 { font-size: 23px; }
      .podium-count { font-size: 30px; }
      .medal { width: 54px; height: 54px; font-size: 26px; }
      .rank-row { grid-template-columns: 35px minmax(72px, .75fr) 1.5fr 42px; gap: 8px; font-size: 15px; }
    }
    @media print {
      body { background: var(--page); print-color-adjust: exact; -webkit-print-color-adjust: exact; }
      .report { width: 1080px; min-height: 1350px; }
    }
  </style>
</head>
<body>
  <main class="report">
    <header>
      <div class="wordmark">cmdfreq</div>
      <h1>Your command line, ranked.</h1>
      <p class="subtitle">A snapshot of your shell habits · {{.Period}}</p>
    </header>

    <section class="stats" aria-label="Summary statistics">
      <div class="stat"><strong>{{.TotalCommands}}</strong><span>Commands</span></div>
      <div class="stat"><strong>{{.UniqueCommands}}</strong><span>Unique</span></div>
      <div class="stat"><strong>{{.TopCount}}</strong><span>Top count</span></div>
    </section>

    <section aria-labelledby="podium-title">
      <h2 class="section-title" id="podium-title">The podium</h2>
      <div class="podium">
        {{range .Podium}}
        <article class="podium-card {{.Place}}">
          <div class="medal" aria-label="Rank {{.Rank}}">{{.Rank}}</div>
          <h2>{{.Name}}</h2>
          <div class="podium-count">{{.Count}}<small>uses</small></div>
          <div class="podium-percent">{{.Percentage}}</div>
        </article>
        {{end}}
      </div>
    </section>

    {{if .Remaining}}
    <section aria-labelledby="top-ten-title">
      <h2 class="section-title" id="top-ten-title">The top 10</h2>
      <div class="ranking">
        {{range .Remaining}}
        <div class="rank-row">
          <span class="rank-number">{{.RankLabel}}</span>
          <span class="command-name">{{.Name}}</span>
          <span class="bar-track" aria-hidden="true"><span class="bar" style="width: {{.BarWidth}}%"></span></span>
          <span class="rank-count">{{.Count}}</span>
        </div>
        {{end}}
      </div>
    </section>
    {{end}}

    <footer>
      Generated locally · Your shell history never leaves your machine
      <strong>cmdfreq</strong>
    </footer>
  </main>
</body>
</html>`))
