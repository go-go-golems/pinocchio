import type { ProfileInfo } from '../../../store/profileApi';
import type { ChatWidgetProps } from '../../../webchat/types';

export type WebChatAppProps = ChatWidgetProps & {
  selectedProfile: string;
  profileOptions: ProfileInfo[];
  profileTitle: string;
  onProfileChange: (slug: string) => void;
};
