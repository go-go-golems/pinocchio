import type { BaseQueryFn, FetchArgs, FetchBaseQueryError } from '@reduxjs/toolkit/query';
import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import { basePrefixFromLocation } from '../utils/basePrefix';

export type ProfileInfo = {
  slug: string;
  default_prompt?: string;
};

export type CurrentProfile = {
  slug: string;
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

export const profileApi = createApi({
  reducerPath: 'profileApi',
  baseQuery,
  tagTypes: ['Profile'],
  endpoints: (builder) => ({
    getProfiles: builder.query<ProfileInfo[], void>({
      query: () => '/api/chat/profiles',
    }),
    getProfile: builder.query<CurrentProfile, void>({
      query: () => '/api/chat/profile',
      providesTags: ['Profile'],
    }),
    setProfile: builder.mutation<CurrentProfile, { slug: string }>({
      query: (body) => ({
        url: '/api/chat/profile',
        method: 'POST',
        body,
      }),
      invalidatesTags: ['Profile'],
    }),
  }),
});

export const { useGetProfilesQuery, useGetProfileQuery, useSetProfileMutation } = profileApi;
