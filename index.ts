import Elysia from "elysia";
import { z } from "zod";

const GOTOSOCIAL_URL = Bun.env.GOTOSOCIAL_URL;
if (!GOTOSOCIAL_URL) {
	throw new Error("GOTOSOCIAL_URL is not set in .env file");
}

const TOKEN = Bun.env.TOKEN;
if (!TOKEN) {
	throw new Error("TOKEN is not set in .env file");
}

let sitemap: {
	content: string;
	lastUpdatedAt: Date;
} | null = null;

async function getSitemap() {
	if (
		sitemap &&
		Date.now() - sitemap.lastUpdatedAt.getTime() < 60 * 60 * 1000
	) {
		return sitemap.content;
	}

	const tootsSchema = z.array(
		z.object({
			id: z.string(),
			created_at: z.iso.datetime(),
			url: z.url(),
		}),
	);

	let content = "";
	content += `<?xml version="1.0" encoding="UTF-8" ?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`;

	let after: string | undefined = undefined;
	do {
		const toots: z.infer<typeof tootsSchema> = await fetch(
			GOTOSOCIAL_URL + `/api/v1/timelines/public?local=true&max_id=${after}`,
			{
				headers: {
					Authorization: `Bearer ${TOKEN}`,
				},
			},
		)
			.then((res) => res.json())
			.then((res) => tootsSchema.parse(res));

		toots.forEach((toot) => {
			content += `<url><loc>${toot.url}</loc><lastmod>${toot.created_at}</lastmod></url>`;
		});

		after = toots.pop()?.id;
	} while (after);

	content += `</urlset>`;

	sitemap = {
		content,
		lastUpdatedAt: new Date(),
	};

	return content;
}

new Elysia()
	.onError(({ error, code }) => {
		switch (code) {
			case "NOT_FOUND":
				return new Response(null, { status: 404 });
			case "INTERNAL_SERVER_ERROR":
			case "UNKNOWN":
				console.error(error);
				return new Response(null, { status: 500 });
			case "INVALID_COOKIE_SIGNATURE":
			case "INVALID_FILE_TYPE":
			case "PARSE":
			case "VALIDATION":
				return new Response(null, { status: 400 });
		}
	})
	.get("/sitemap.xml", async ({ set }) => {
		set.headers["content-type"] = "application/xml";
		return await getSitemap();
	})
	.listen(
		{
			port: Bun.env.PORT,
		},
		({ protocol, hostname, port }) => {
			console.log(`Server is running on ${protocol}://${hostname}:${port}`);
		},
	);
