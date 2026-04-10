import { ImageResponse } from "next/og";

export const runtime = "edge";

export async function GET() {
	return new ImageResponse(
		(
			<div
				style={{
					width: "100%",
					height: "100%",
					display: "flex",
					flexDirection: "column",
					alignItems: "center",
					justifyContent: "center",
					background: "linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #0f172a 100%)",
					position: "relative",
				}}
			>
				{/* Gradient orbs */}
				<div
					style={{
						position: "absolute",
						top: -100,
						left: 100,
						width: 500,
						height: 500,
						borderRadius: "50%",
						background: "radial-gradient(circle, rgba(16,163,127,0.25) 0%, transparent 70%)",
					}}
				/>
				<div
					style={{
						position: "absolute",
						bottom: -100,
						right: 100,
						width: 500,
						height: 500,
						borderRadius: "50%",
						background: "radial-gradient(circle, rgba(217,119,87,0.25) 0%, transparent 70%)",
					}}
				/>

				{/* Brand */}
				<div
					style={{
						display: "flex",
						alignItems: "center",
						marginBottom: 32,
					}}
				>
					<span
						style={{
							fontSize: 36,
							fontWeight: 800,
							color: "#f1f5f9",
							letterSpacing: "-0.02em",
						}}
					>
						Vibe
					</span>
					<span
						style={{
							fontSize: 36,
							fontWeight: 800,
							color: "#94a3b8",
							letterSpacing: "-0.02em",
						}}
					>
						Tradez
					</span>
				</div>

				{/* Headline */}
				<div
					style={{
						display: "flex",
						flexDirection: "column",
						alignItems: "center",
						gap: 8,
					}}
				>
					<span
						style={{
							fontSize: 64,
							fontWeight: 800,
							color: "#f1f5f9",
							letterSpacing: "-0.03em",
						}}
					>
						Two AIs.
					</span>
					<span
						style={{
							fontSize: 64,
							fontWeight: 800,
							letterSpacing: "-0.03em",
							background: "linear-gradient(90deg, #10a37f, #D97757)",
							backgroundClip: "text",
							color: "transparent",
						}}
					>
						One Trade List.
					</span>
				</div>

				{/* Subline */}
				<div
					style={{
						fontSize: 24,
						color: "#94a3b8",
						marginTop: 24,
						textAlign: "center",
						maxWidth: 800,
					}}
				>
					Free daily ranked options picks powered by GPT-5.4 + Claude
					Opus 4.6
				</div>

				{/* Model badges */}
				<div
					style={{
						display: "flex",
						gap: 40,
						marginTop: 48,
						alignItems: "center",
					}}
				>
					<div
						style={{
							display: "flex",
							alignItems: "center",
							gap: 10,
						}}
					>
						<div
							style={{
								width: 12,
								height: 12,
								borderRadius: "50%",
								background: "#10a37f",
							}}
						/>
						<span style={{ color: "#94a3b8", fontSize: 20 }}>
							OpenAI GPT-5.4
						</span>
					</div>
					<div
						style={{
							width: 1,
							height: 24,
							background: "#334155",
						}}
					/>
					<div
						style={{
							display: "flex",
							alignItems: "center",
							gap: 10,
						}}
					>
						<div
							style={{
								width: 12,
								height: 12,
								borderRadius: "50%",
								background: "#D97757",
							}}
						/>
						<span style={{ color: "#94a3b8", fontSize: 20 }}>
							Claude Opus 4.6
						</span>
					</div>
				</div>
			</div>
		),
		{
			width: 1200,
			height: 630,
		},
	);
}
