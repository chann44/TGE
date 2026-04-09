import { ImageResponse } from "next/og";

export const alt = "Arrakis — Know what you run.";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

async function loadDotGothicFont(): Promise<ArrayBuffer> {
  // Fetch CSS from Google Fonts with a legacy UA to get TTF src URLs
  const css = await fetch(
    "https://fonts.googleapis.com/css2?family=DotGothic16&display=swap",
    {
      headers: {
        "User-Agent":
          "Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 Chrome/41 Safari/537.36",
      },
    }
  ).then((r) => r.text());

  const url = css.match(/url\(([^)]+)\)/)?.[1];
  if (!url) throw new Error("Could not extract font URL");
  return fetch(url).then((r) => r.arrayBuffer());
}

export default async function Image() {
  const fontData = await loadDotGothicFont();

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          background: "#1D1818",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          alignItems: "flex-start",
          padding: "80px 96px",
          position: "relative",
          fontFamily: "DotGothic16",
        }}
      >
        {/* Dot grid overlay via SVG */}
        <svg
          width="1200"
          height="630"
          style={{ position: "absolute", top: 0, left: 0 }}
        >
          <defs>
            <pattern
              id="dots"
              x="0"
              y="0"
              width="24"
              height="24"
              patternUnits="userSpaceOnUse"
            >
              <circle cx="1" cy="1" r="1" fill="rgba(255,255,255,0.12)" />
            </pattern>
          </defs>
          <rect width="1200" height="630" fill="url(#dots)" />
        </svg>

        {/* Top-left status tag */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "10px",
            marginBottom: "64px",
            position: "relative",
          }}
        >
          <div
            style={{
              width: "8px",
              height: "8px",
              borderRadius: "50%",
              background: "#FAD9D3",
            }}
          />
          <span
            style={{
              fontFamily: "DotGothic16",
              fontSize: "16px",
              letterSpacing: "0.15em",
              textTransform: "uppercase",
              color: "#959190",
            }}
          >
            SYS.ONLINE // OPEN SOURCE
          </span>
        </div>

        {/* Star motif */}
        <svg
          width="40"
          height="40"
          viewBox="0 0 24 24"
          fill="#FAD9D3"
          style={{ marginBottom: "28px", position: "relative" }}
        >
          <path d="M12 0L13.5 10.5L24 12L13.5 13.5L12 24L10.5 13.5L0 12L10.5 10.5L12 0Z" />
        </svg>

        {/* Main heading */}
        <div
          style={{
            fontFamily: "DotGothic16",
            fontSize: "128px",
            lineHeight: 1,
            color: "#FAD9D3",
            letterSpacing: "-0.01em",
            position: "relative",
            marginBottom: "28px",
          }}
        >
          ARRAKIS
        </div>

        {/* Subtitle */}
        <div
          style={{
            fontFamily: "DotGothic16",
            fontSize: "28px",
            color: "#959190",
            letterSpacing: "0.04em",
            position: "relative",
          }}
        >
          Know what you run.
        </div>

        {/* Bottom-right decorative text */}
        <div
          style={{
            position: "absolute",
            bottom: "80px",
            right: "96px",
            fontFamily: "DotGothic16",
            fontSize: "13px",
            letterSpacing: "0.12em",
            color: "rgba(255,255,255,0.15)",
            textTransform: "uppercase",
          }}
        >
          Supply Chain Security // Self-Hosted
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [
        {
          name: "DotGothic16",
          data: fontData,
          style: "normal",
          weight: 400,
        },
      ],
    }
  );
}
