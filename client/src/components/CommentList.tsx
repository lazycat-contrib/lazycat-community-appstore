import { type FormEvent } from 'react';
import { MessageSquare, Trash2 } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { EmptyState } from '../shared/components/Feedback';
import type { Comment } from '../shared/types';
import { formatDate } from '../shared/utils';

export function CommentList({
  comments,
  commentsState = 'loaded',
  canReply = true,
  replyTarget,
  replyText,
  onReplyTarget,
  onReplyText,
  onReply,
  onDelete,
}: {
  comments: Comment[];
  commentsState?: 'idle' | 'loading' | 'loaded' | 'error';
  canReply?: boolean;
  replyTarget: number | null;
  replyText: string;
  onReplyTarget: (id: number | null) => void;
  onReplyText: (value: string) => void;
  onReply: (event: FormEvent, parentId: number) => void;
  onDelete: (id: number) => void;
}) {
  const { t } = useTranslation();
  if (commentsState === 'loading') {
    return (
      <div className="comments">
        <div className="comment skeleton-comment" aria-label={t('common.loading')} />
      </div>
    );
  }
  if (comments.length === 0) {
    return <EmptyState icon={MessageSquare} title={t('drawer.noComments')} body={t('drawer.noCommentsBody')} />;
  }
  return (
    <div className="comments">
      {comments.map((comment) => (
        <article className="comment" key={comment.id}>
          <CommentBody comment={comment} onDelete={onDelete} />
          {canReply && (
            <div className="comment-actions">
              <XButton type="button" variant="secondary" size="sm" label={t('drawer.reply')} icon={<MessageSquare size={15} />} onClick={() => onReplyTarget(replyTarget === comment.id ? null : comment.id)} />
            </div>
          )}
          {canReply && replyTarget === comment.id && (
            <form className="comment-form rich-comment-form reply-form" onSubmit={(event) => onReply(event, comment.id)}>
              <XTextInput
                label={t('drawer.replyPlaceholder')}
                isLabelHidden
                value={replyText}
                placeholder={t('drawer.replyPlaceholder')}
                onChange={onReplyText}
              />
              <XIconButton type="submit" variant="ghost" label={t('drawer.postReply')} icon={<MessageSquare size={17} />} isDisabled={!replyText.trim()} />
            </form>
          )}
          {comment.replies && comment.replies.length > 0 && (
            <div className="comment-replies">
              {comment.replies.map((reply) => (
                <article className="comment reply" key={reply.id}>
                  <CommentBody comment={reply} onDelete={onDelete} />
                </article>
              ))}
            </div>
          )}
        </article>
      ))}
    </div>
  );
}

function CommentBody({ comment, onDelete }: { comment: Comment; onDelete: (id: number) => void }) {
  const { t } = useTranslation();
  return (
    <>
      <div className="comment-head">
        <div>
          <strong>{comment.username}</strong>
          <span>{formatDate(comment.createdAt)}</span>
        </div>
        {comment.canDelete && (
          <XIconButton type="button" variant="destructive" label={t('drawer.deleteComment')} icon={<Trash2 size={15} />} onClick={() => onDelete(comment.id)} />
        )}
      </div>
      <p>{comment.body}</p>
    </>
  );
}
