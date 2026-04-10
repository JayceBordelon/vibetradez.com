import type { MetadataRoute } from "next";

export default function sitemap(): MetadataRoute.Sitemap {
	return [
		{
			url: "https://vibetradez.com",
			changeFrequency: "daily",
			priority: 1.0,
		},
		{
			url: "https://vibetradez.com/history",
			changeFrequency: "daily",
			priority: 0.8,
		},
		{
			url: "https://vibetradez.com/terms",
			changeFrequency: "monthly",
			priority: 0.3,
		},
		{
			url: "https://vibetradez.com/faq",
			changeFrequency: "monthly",
			priority: 0.5,
		},
	];
}
