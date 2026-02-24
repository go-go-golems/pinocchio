import type { BaseQueryFn, FetchArgs, FetchBaseQueryError } from '@reduxjs/toolkit/query';
import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import { basePrefixFromLocation } from '../utils/basePrefix';

export type ProfileInfo = {
  slug: string;
  display_name?: string;
  description?: string;
  default_prompt?: string;
  is_default?: boolean;
};

export type CurrentProfile = {
  slug: string;
  profile?: string;
};

const rawBaseQuery = fetchBaseQuery({ baseUrl: '' });

const baseQuery: BaseQueryFn<string | FetchArgs, unknown, FetchBaseQueryError> = async (args, api, extraOptions) => {
  const prefix = basePrefixFromLocation();
  const nextArgs =
    typeof args === 'string'
      ? `${prefix}${args}`
      : {
          ...args,
          url: `${prefix}${args.url}`,
        };
  return rawBaseQuery(nextArgs, api, extraOptions);
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function decodeProfileInfo(payload: unknown, index: number): ProfileInfo {
  if (!isRecord(payload)) {
    throw new Error(`invalid profile list item at index ${index}`);
  }
  const slug = typeof payload.slug === 'string' ? payload.slug.trim() : '';
  if (!slug) {
    throw new Error(`invalid profile slug at index ${index}`);
  }
  const out: ProfileInfo = { slug };
  if (typeof payload.display_name === 'string') out.display_name = payload.display_name;
  if (typeof payload.description === 'string') out.description = payload.description;
  if (typeof payload.default_prompt === 'string') out.default_prompt = payload.default_prompt;
  if (typeof payload.is_default === 'boolean') out.is_default = payload.is_default;
  return out;
}

function decodeProfilesResponse(payload: unknown): ProfileInfo[] {
  if (Array.isArray(payload)) {
    return payload.map((item, index) => decodeProfileInfo(item, index));
  }
  if (!isRecord(payload)) {
    throw new Error('invalid profile list response: expected array');
  }
  const keys = Object.keys(payload);
  if (keys.length === 0) {
    return [];
  }
  if (!keys.every((key) => /^[0-9]+$/.test(key))) {
    throw new Error('invalid profile list response: expected array');
  }
  return keys
    .sort((a, b) => Number(a) - Number(b))
    .map((key, index) => decodeProfileInfo(payload[key], index));
}

function decodeCurrentProfile(payload: unknown): CurrentProfile {
  if (!isRecord(payload)) {
    throw new Error('invalid current profile response');
  }
  const slug = typeof payload.slug === 'string' ? payload.slug.trim() : '';
  const fallbackProfile = typeof payload.profile === 'string' ? payload.profile.trim() : '';
  const resolvedSlug = slug || fallbackProfile;
  if (!resolvedSlug) {
    throw new Error('invalid current profile response');
  }
  const out: CurrentProfile = { slug: resolvedSlug };
  if (fallbackProfile) {
    out.profile = fallbackProfile;
  }
  return out;
}

export const profileApi = createApi({
  reducerPath: 'profileApi',
  baseQuery,
  tagTypes: ['Profile'],
  endpoints: (builder) => ({
    getProfiles: builder.query<ProfileInfo[], void>({
      query: () => '/api/chat/profiles',
      transformResponse: decodeProfilesResponse,
    }),
    getProfile: builder.query<CurrentProfile, void>({
      query: () => '/api/chat/profile',
      providesTags: ['Profile'],
      transformResponse: decodeCurrentProfile,
    }),
    setProfile: builder.mutation<CurrentProfile, { slug: string }>({
      query: (body) => ({
        url: '/api/chat/profile',
        method: 'POST',
        body,
      }),
      invalidatesTags: ['Profile'],
      transformResponse: decodeCurrentProfile,
    }),
  }),
});

export const { useGetProfilesQuery, useGetProfileQuery, useSetProfileMutation } = profileApi;
